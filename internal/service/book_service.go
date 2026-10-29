package service

import (
	"context"
	"errors"
	"os"

	"booksReader/internal/dto"
	"booksReader/internal/models"
	"booksReader/internal/repository"

	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"

	"booksReader/internal/worker"

	"github.com/hibiken/asynq"
)

type BookService interface {
	Create(ctx context.Context, title string, author string, year int, pdfPath string, userID int) error
	GetAll(ctx context.Context) ([]dto.BookSummaryResponse, error)
	GetDetail(ctx context.Context, id int) (dto.BookDetailResponse, error)
	GetContent(ctx context.Context, id int) (string, error)
	Update(ctx context.Context, id int, req dto.BookRequest, userID int) error
	UpdateProgress(ctx context.Context, id int, page int, userID int) error
	Delete(ctx context.Context, id int, userID int) error
}

type bookService struct {
	repo  repository.BookRepository
	redis *redis.Client
	asynq *asynq.Client
}

func NewBookService(repo repository.BookRepository, rdb *redis.Client, asynqClient *asynq.Client) BookService {
	return &bookService{repo: repo, redis: rdb, asynq: asynqClient}
}

func (s *bookService) Create(ctx context.Context, title string, author string, year int, pdfPath string, userID int) error {
	book := models.Book{
		UserID:        userID,
		Title:         title,
		Author:        author,
		PublishedYear: year,
		PdfPath:       pdfPath,
		ThumbnailPath: "", // Dikosongkan sementara, nanti diisi oleh Worker
	}

	// 1. Simpan ke DB (Cepat)
	bookID, err := s.repo.Create(ctx, book)
	if err != nil {
		return err
	}

	// 2. Hapus Cache List Buku
	s.redis.Del(ctx, "books:all")

	// 3. KIRIM TUGAS KE REDIS ASYNQ (Sangat Cepat, < 1ms)
	// Kita beritahu Worker: "Tolong proses PDF ini untuk BookID ini"
	task, err := worker.NewGenerateThumbnailTask(bookID, "./uploads/pdf/"+pdfPath, pdfPath)
	if err == nil {
		s.asynq.Enqueue(task) // Kirim ke Antrean!
	}

	return nil
}

func (s *bookService) GetAll(ctx context.Context) ([]dto.BookSummaryResponse, error) {
	cacheKey := "books:all"

	// 1. CEK REDIS: Apakah data buku sudah ada di RAM?
	cachedData, err := s.redis.Get(ctx, cacheKey).Result()
	if err == nil {
		// Jika ada, langsung ubah dari String (JSON) kembali ke bentuk Array Go
		var results []dto.BookSummaryResponse
		json.Unmarshal([]byte(cachedData), &results)
		return results, nil // Selesai! Tidak perlu query ke PostgreSQL
	}

	// 2. JIKA REDIS KOSONG: Ambil dari PostgreSQL
	books, err := s.repo.GetAll(ctx)
	if err != nil {
		return nil, err
	}

	var results []dto.BookSummaryResponse
	for _, b := range books {
		results = append(results, dto.BookSummaryResponse{
			ID:            b.ID,
			Title:         b.Title,
			Author:        b.Author,
			PublishedYear: b.PublishedYear,
			ThumbnailUrl:  "/uploads/thumbnails/" + b.ThumbnailPath,
		})
	}
	booksJson, _ := json.Marshal(results)
	s.redis.Set(ctx, cacheKey, booksJson, 5*time.Minute)

	return results, nil
}

// Get Detail Info
func (s *bookService) GetDetail(ctx context.Context, id int) (dto.BookDetailResponse, error) {
	b, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return dto.BookDetailResponse{}, err
	}

	// PERBAIKAN: Kita gabungkan path agar menjadi URL yang bisa diakses
	fullPdfUrl := "/uploads/pdf/" + b.PdfPath

	return dto.BookDetailResponse{
		ID:            b.ID,
		Title:         b.Title,
		Author:        b.Author,
		PublishedYear: b.PublishedYear,
		PdfUrl:        fullPdfUrl, // Sekarang sudah berbentuk URL siap pakai
		ThumbnailUrl:  "/uploads/thumbnails/" + b.ThumbnailPath,
		LastReadPage:  b.LastReadPage,
	}, nil
}

// Khusus mengambil URL PDF (Pengganti Read Content teks)
func (s *bookService) GetContent(ctx context.Context, id int) (string, error) {
	b, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return "", err
	}

	// PERBAIKAN: Kembalikan URL file PDF
	fullPdfUrl := "/uploads/pdf/" + b.PdfPath
	return fullPdfUrl, nil
}

// Update (Hanya Update Info Metadata Buku)
func (s *bookService) Update(ctx context.Context, id int, req dto.BookRequest, userID int) error {
	existingBook, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if existingBook.UserID != userID {
		return errors.New("anda tidak memiliki izin untuk mengubah buku ini")
	}

	existingBook.Title = req.Title
	existingBook.Author = req.Author
	existingBook.PublishedYear = req.PublishedYear

	err = s.repo.Update(ctx, existingBook)
	if err == nil {
		// PERBAIKAN: Hapus Cache karena judul/tahun/author buku berubah!
		s.redis.Del(ctx, "books:all")
	}

	return err
}

func (s *bookService) UpdateProgress(ctx context.Context, id int, page int, userID int) error {
	// Validasi halaman
	if page < 1 {
		return errors.New("nomor halaman tidak valid")
	}

	// Cek apakah buku ada dan milik user
	existingBook, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if existingBook.UserID != userID {
		return errors.New("anda tidak memiliki izin")
	}

	// Simpan ke DB
	return s.repo.UpdateProgress(ctx, id, page)
}

// Delete Buku
func (s *bookService) Delete(ctx context.Context, id int, userID int) error {
	existingBook, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if existingBook.UserID != userID {
		return errors.New("anda tidak memiliki izin untuk menghapus buku ini")
	}

	// 1. Hapus data dari Database PostgreSQL
	err = s.repo.Delete(ctx, id)
	if err != nil {
		return err
	}

	// 2. Hapus file PDF fisik dari Harddisk Server
	filePath := "./uploads/pdf/" + existingBook.PdfPath
	os.Remove(filePath)

	// 3. PERBAIKAN: Langsung hapus cache (kode s.repo.Delete ganda sudah dihapus)
	s.redis.Del(ctx, "books:all")

	return nil
}
