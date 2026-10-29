package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"booksReader/internal/dto"
	"booksReader/internal/models"
	"booksReader/internal/repository"
	"booksReader/internal/utils"
	"booksReader/internal/worker"

	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
)

// Error umum yang dipetakan handler ke status HTTP (lihat handlers.sendServiceError)
var (
	ErrNotFound  = errors.New("Buku tidak ditemukan")                    // 404
	ErrForbidden = errors.New("anda tidak memiliki izin untuk buku ini") // 403
)

// ValidationError berarti input dari user salah (400). Pesannya aman ditampilkan ke user.
type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

func newValidationError(format string, args ...interface{}) error {
	return &ValidationError{Message: fmt.Sprintf(format, args...)}
}

const (
	maxListLimit     = 100
	defaultListLimit = 20
	maxSearchLength  = 100
	booksCacheTTL    = 5 * time.Minute
	pdfDir           = "./uploads/pdf/"
	thumbnailDir     = "./uploads/thumbnails/"
)

type BookService interface {
	Create(ctx context.Context, title string, author string, year int, pdfFileName string, userID int) (dto.BookDetailResponse, error)
	GetAll(ctx context.Context, userID int) ([]dto.BookSummaryResponse, error)
	List(ctx context.Context, userID int, params dto.BookListParams) ([]dto.BookSummaryResponse, dto.PaginationMeta, error)
	GetDetail(ctx context.Context, id int, userID int) (dto.BookDetailResponse, error)
	GetContent(ctx context.Context, id int, userID int) (dto.BookContentResponse, error)
	GetPdfFilePath(ctx context.Context, id int, userID int) (string, error)
	Update(ctx context.Context, id int, req dto.UpdateBookRequest, userID int) (dto.BookDetailResponse, error)
	UpdateProgress(ctx context.Context, id int, page int, userID int) error
	SetFavorite(ctx context.Context, id int, isFavorite bool, userID int) (dto.BookDetailResponse, error)
	Delete(ctx context.Context, id int, userID int) error
}

type bookService struct {
	repo  repository.BookRepository
	redis *redis.Client
	asynq *asynq.Client
}

// Cache daftar buku dipisah per user
func booksCacheKey(userID int) string {
	return fmt.Sprintf("books:user:%d", userID)
}

func NewBookService(repo repository.BookRepository, rdb *redis.Client, asynqClient *asynq.Client) BookService {
	return &bookService{repo: repo, redis: rdb, asynq: asynqClient}
}

// ValidateBookInput dipakai saat upload (handler & service) dan saat edit metadata
func ValidateBookInput(title string, author string, year int) error {
	if strings.TrimSpace(title) == "" {
		return newValidationError("Judul buku tidak boleh kosong")
	}
	if strings.TrimSpace(author) == "" {
		return newValidationError("Nama pengarang tidak boleh kosong")
	}
	if maxYear := time.Now().Year(); year < 1000 || year > maxYear {
		return newValidationError("Tahun terbit harus di antara 1000 dan %d", maxYear)
	}
	return nil
}

func thumbnailURL(thumbnailPath string) string {
	if thumbnailPath == "" {
		return "" // Sampul belum dibuat worker
	}
	return "/uploads/thumbnails/" + thumbnailPath
}

// PDF tidak lagi di-serve sebagai file statis, tapi lewat endpoint yang mengecek pemilik buku
func pdfURL(bookID int) string {
	return fmt.Sprintf("/api/v1/books/%d/file", bookID)
}

func toDetail(b models.Book) dto.BookDetailResponse {
	return dto.BookDetailResponse{
		ID:            b.ID,
		Title:         b.Title,
		Author:        b.Author,
		PublishedYear: b.PublishedYear,
		PdfUrl:        pdfURL(b.ID),
		ThumbnailUrl:  thumbnailURL(b.ThumbnailPath),
		LastReadPage:  b.LastReadPage,
		TotalPages:    b.TotalPages,
		IsFavorite:    b.IsFavorite,
		FinishedAt:    b.FinishedAt,
	}
}

func (s *bookService) Create(ctx context.Context, title string, author string, year int, pdfFileName string, userID int) (dto.BookDetailResponse, error) {
	if err := ValidateBookInput(title, author, year); err != nil {
		return dto.BookDetailResponse{}, err
	}

	// Hitung jumlah halaman sekarang (cepat). Jika gagal, worker akan mencoba lagi.
	totalPages, err := utils.GetPdfPageCount(pdfDir + pdfFileName)
	if err != nil {
		totalPages = 0
	}

	book := models.Book{
		UserID:        userID,
		Title:         strings.TrimSpace(title),
		Author:        strings.TrimSpace(author),
		PublishedYear: year,
		PdfPath:       pdfFileName,
		ThumbnailPath: "", // Dikosongkan sementara, nanti diisi oleh Worker
		TotalPages:    totalPages,
	}

	// 1. Simpan ke DB (Cepat)
	bookID, err := s.repo.Create(ctx, book)
	if err != nil {
		return dto.BookDetailResponse{}, err
	}
	book.ID = bookID

	// 2. Hapus Cache List Buku milik user ini
	s.redis.Del(ctx, booksCacheKey(userID))

	// 3. KIRIM TUGAS KE REDIS ASYNQ (Sangat Cepat, < 1ms)
	task, err := worker.NewGenerateThumbnailTask(bookID, userID, pdfDir+pdfFileName, pdfFileName)
	if err == nil {
		s.asynq.Enqueue(task) // Kirim ke Antrean!
	}

	return toDetail(book), nil
}

// GetAll mengembalikan semua buku milik user (perilaku lama tanpa paginasi, memakai cache Redis)
func (s *bookService) GetAll(ctx context.Context, userID int) ([]dto.BookSummaryResponse, error) {
	cacheKey := booksCacheKey(userID)

	// 1. CEK REDIS: Apakah data buku sudah ada di RAM?
	cachedData, err := s.redis.Get(ctx, cacheKey).Result()
	if err == nil {
		var results []dto.BookSummaryResponse
		if json.Unmarshal([]byte(cachedData), &results) == nil {
			return results, nil // Selesai! Tidak perlu query ke PostgreSQL
		}
	}

	// 2. JIKA REDIS KOSONG: Ambil dari PostgreSQL
	books, err := s.repo.GetAllByUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	results := s.toSummaries(ctx, books)
	booksJson, _ := json.Marshal(results)
	s.redis.Set(ctx, cacheKey, booksJson, booksCacheTTL)

	return results, nil
}

// List adalah GET /books dengan pencarian, sorting, dan paginasi (tidak di-cache)
func (s *bookService) List(ctx context.Context, userID int, params dto.BookListParams) ([]dto.BookSummaryResponse, dto.PaginationMeta, error) {
	if params.Page == 0 {
		params.Page = 1
	}
	if params.Limit == 0 {
		params.Limit = defaultListLimit
	}
	if params.Sort == "" {
		params.Sort = "-created_at"
	}

	if params.Page < 1 {
		return nil, dto.PaginationMeta{}, newValidationError("Parameter 'page' minimal 1")
	}
	if params.Limit < 1 || params.Limit > maxListLimit {
		return nil, dto.PaginationMeta{}, newValidationError("Parameter 'limit' harus di antara 1 dan %d", maxListLimit)
	}

	search := strings.TrimSpace(params.Q)
	if utf8.RuneCountInString(search) > maxSearchLength {
		return nil, dto.PaginationMeta{}, newValidationError("Parameter 'q' maksimal %d karakter", maxSearchLength)
	}

	// Format sort: "title" (naik) atau "-title" (turun)
	sortColumn, desc := strings.CutPrefix(params.Sort, "-")
	if _, ok := repository.SortableColumns[sortColumn]; !ok {
		return nil, dto.PaginationMeta{}, newValidationError("Parameter 'sort' tidak valid. Pilihan: created_at, title, author, published_year (awali dengan '-' untuk urutan menurun)")
	}

	books, total, err := s.repo.ListByUser(ctx, userID, repository.BookListQuery{
		Search:     search,
		SortColumn: sortColumn,
		Desc:       desc,
		Limit:      params.Limit,
		Offset:     (params.Page - 1) * params.Limit,
	})
	if err != nil {
		return nil, dto.PaginationMeta{}, err
	}

	meta := dto.PaginationMeta{
		Page:       params.Page,
		Limit:      params.Limit,
		TotalItems: total,
		HasNext:    params.Page*params.Limit < total,
	}
	return s.toSummaries(ctx, books), meta, nil
}

func (s *bookService) toSummaries(ctx context.Context, books []models.Book) []dto.BookSummaryResponse {
	results := make([]dto.BookSummaryResponse, 0, len(books))
	for _, b := range books {
		// Buku lama (sebelum ada kolom total_pages): hitung sekali lalu simpan
		if b.TotalPages == 0 && b.PdfPath != "" {
			if pages, err := utils.GetPdfPageCount(pdfDir + b.PdfPath); err == nil {
				b.TotalPages = pages
				s.repo.UpdateTotalPages(ctx, b.ID, pages)
			}
		}

		results = append(results, dto.BookSummaryResponse{
			ID:            b.ID,
			Title:         b.Title,
			Author:        b.Author,
			PublishedYear: b.PublishedYear,
			ThumbnailUrl:  thumbnailURL(b.ThumbnailPath),
			LastReadPage:  b.LastReadPage,
			TotalPages:    b.TotalPages,
			IsFavorite:    b.IsFavorite,
			FinishedAt:    b.FinishedAt,
		})
	}
	return results
}

// findOwnedBook untuk akses baca: buku milik user lain dianggap tidak ada (404),
// sama seperti repository.ErrBookNotFound untuk ID yang memang tidak ada,
// supaya ID buku orang lain tidak bisa ditebak.
func findOwnedBook(ctx context.Context, repo repository.BookRepository, id int, userID int) (models.Book, error) {
	b, err := repo.GetByID(ctx, id)
	if err != nil {
		return b, err
	}
	if b.UserID != userID {
		return b, ErrNotFound
	}
	return b, nil
}

// findBookForWrite untuk akses ubah/hapus: buku milik user lain mengembalikan 403
func findBookForWrite(ctx context.Context, repo repository.BookRepository, id int, userID int) (models.Book, error) {
	b, err := repo.GetByID(ctx, id)
	if err != nil {
		return b, err
	}
	if b.UserID != userID {
		return b, ErrForbidden
	}
	return b, nil
}

func (s *bookService) GetDetail(ctx context.Context, id int, userID int) (dto.BookDetailResponse, error) {
	b, err := findOwnedBook(ctx, s.repo, id, userID)
	if err != nil {
		return dto.BookDetailResponse{}, err
	}
	return toDetail(b), nil
}

func (s *bookService) GetContent(ctx context.Context, id int, userID int) (dto.BookContentResponse, error) {
	b, err := findOwnedBook(ctx, s.repo, id, userID)
	if err != nil {
		return dto.BookContentResponse{}, err
	}
	return dto.BookContentResponse{
		PdfUrl:       pdfURL(b.ID),
		LastReadPage: b.LastReadPage,
		TotalPages:   b.TotalPages,
	}, nil
}

// GetPdfFilePath mengembalikan lokasi file PDF di disk (untuk GET /books/{id}/file)
func (s *bookService) GetPdfFilePath(ctx context.Context, id int, userID int) (string, error) {
	b, err := findOwnedBook(ctx, s.repo, id, userID)
	if err != nil {
		return "", err
	}
	return pdfDir + b.PdfPath, nil
}

// Update dipakai oleh PUT (semua field diisi handler) dan PATCH (field nil = tidak diubah)
func (s *bookService) Update(ctx context.Context, id int, req dto.UpdateBookRequest, userID int) (dto.BookDetailResponse, error) {
	b, err := findBookForWrite(ctx, s.repo, id, userID)
	if err != nil {
		return dto.BookDetailResponse{}, err
	}

	if req.Title == nil && req.Author == nil && req.PublishedYear == nil {
		return dto.BookDetailResponse{}, newValidationError("Tidak ada data yang diubah")
	}

	if req.Title != nil {
		b.Title = strings.TrimSpace(*req.Title)
	}
	if req.Author != nil {
		b.Author = strings.TrimSpace(*req.Author)
	}
	if req.PublishedYear != nil {
		b.PublishedYear = *req.PublishedYear
	}
	if err := ValidateBookInput(b.Title, b.Author, b.PublishedYear); err != nil {
		return dto.BookDetailResponse{}, err
	}

	if err := s.repo.Update(ctx, b); err != nil {
		return dto.BookDetailResponse{}, err
	}

	// Hapus Cache karena judul/tahun/author buku berubah!
	s.redis.Del(ctx, booksCacheKey(userID))

	return toDetail(b), nil
}

func (s *bookService) UpdateProgress(ctx context.Context, id int, page int, userID int) error {
	// Validasi halaman
	if page < 1 {
		return newValidationError("Nomor halaman tidak valid")
	}

	b, err := findBookForWrite(ctx, s.repo, id, userID)
	if err != nil {
		return err
	}
	if b.TotalPages > 0 && page > b.TotalPages {
		return newValidationError("Nomor halaman melebihi total halaman (%d)", b.TotalPages)
	}

	// Sampai halaman terakhir = buku selesai dibaca (finished_at diisi otomatis)
	finished := b.TotalPages > 0 && page == b.TotalPages
	if err := s.repo.UpdateProgress(ctx, id, page, finished); err != nil {
		return err
	}

	s.redis.Del(ctx, booksCacheKey(userID))
	return nil
}

func (s *bookService) SetFavorite(ctx context.Context, id int, isFavorite bool, userID int) (dto.BookDetailResponse, error) {
	b, err := findBookForWrite(ctx, s.repo, id, userID)
	if err != nil {
		return dto.BookDetailResponse{}, err
	}

	if err := s.repo.UpdateFavorite(ctx, id, isFavorite); err != nil {
		return dto.BookDetailResponse{}, err
	}
	b.IsFavorite = isFavorite

	s.redis.Del(ctx, booksCacheKey(userID))
	return toDetail(b), nil
}

// Delete Buku beserta file PDF dan sampulnya
func (s *bookService) Delete(ctx context.Context, id int, userID int) error {
	b, err := findBookForWrite(ctx, s.repo, id, userID)
	if err != nil {
		return err
	}

	// 1. Hapus data dari Database PostgreSQL (bookmark ikut terhapus lewat ON DELETE CASCADE)
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}

	// 2. Hapus file PDF dan sampul dari Harddisk Server
	os.Remove(pdfDir + b.PdfPath)
	if b.ThumbnailPath != "" {
		os.Remove(thumbnailDir + b.ThumbnailPath)
	}

	// 3. Hapus cache daftar buku
	s.redis.Del(ctx, booksCacheKey(userID))

	return nil
}
