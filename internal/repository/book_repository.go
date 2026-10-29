package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"booksReader/internal/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrBookNotFound = errors.New("buku tidak ditemukan")

// Kolom yang boleh dipakai untuk ORDER BY (key = nilai parameter 'sort', value = ekspresi SQL).
// Whitelist ini mencegah SQL injection karena nama kolom tidak bisa dikirim sebagai $param.
var SortableColumns = map[string]string{
	"created_at":     "created_at",
	"title":          "LOWER(title)",
	"author":         "LOWER(author)",
	"published_year": "published_year",
}

const bookColumns = `id, user_id, title, author, published_year, pdf_path, thumbnail_path,
	last_read_page, total_pages, is_favorite, finished_at, created_at`

// BookListQuery adalah parameter GET /books yang sudah divalidasi oleh service
type BookListQuery struct {
	Search     string // Kosong = tanpa pencarian
	SortColumn string // Key dari SortableColumns
	Desc       bool
	Limit      int
	Offset     int
}

type BookRepository interface {
	Create(ctx context.Context, book models.Book) (int, error)
	GetAllByUser(ctx context.Context, userID int) ([]models.Book, error)
	ListByUser(ctx context.Context, userID int, q BookListQuery) ([]models.Book, int, error)
	GetByID(ctx context.Context, id int) (models.Book, error)
	GetWithoutThumbnail(ctx context.Context) ([]models.Book, error)
	Update(ctx context.Context, book models.Book) error
	UpdateProgress(ctx context.Context, id int, page int, finished bool) error
	UpdateFavorite(ctx context.Context, id int, isFavorite bool) error
	UpdateThumbnailPath(ctx context.Context, id int, thumbPath string) error
	UpdateTotalPages(ctx context.Context, id int, totalPages int) error
	Delete(ctx context.Context, id int) error
}

type bookRepository struct {
	db *pgxpool.Pool
}

func NewBookRepository(db *pgxpool.Pool) BookRepository {
	return &bookRepository{db: db}
}

func scanBook(row pgx.Row) (models.Book, error) {
	var b models.Book
	err := row.Scan(&b.ID, &b.UserID, &b.Title, &b.Author, &b.PublishedYear, &b.PdfPath, &b.ThumbnailPath,
		&b.LastReadPage, &b.TotalPages, &b.IsFavorite, &b.FinishedAt, &b.CreatedAt)
	return b, err
}

func (r *bookRepository) queryBooks(ctx context.Context, query string, args ...interface{}) ([]models.Book, error) {
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	books := []models.Book{}
	for rows.Next() {
		b, err := scanBook(rows)
		if err != nil {
			return nil, err
		}
		books = append(books, b)
	}
	return books, rows.Err()
}

func (r *bookRepository) Create(ctx context.Context, book models.Book) (int, error) {
	query := `INSERT INTO books (user_id, title, author, published_year, pdf_path, thumbnail_path, total_pages)
	          VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`

	var insertedID int
	err := r.db.QueryRow(ctx, query, book.UserID, book.Title, book.Author, book.PublishedYear, book.PdfPath, book.ThumbnailPath, book.TotalPages).Scan(&insertedID)
	return insertedID, err
}

func (r *bookRepository) GetAllByUser(ctx context.Context, userID int) ([]models.Book, error) {
	return r.queryBooks(ctx, `SELECT `+bookColumns+` FROM books WHERE user_id = $1 ORDER BY created_at DESC`, userID)
}

// ListByUser mengembalikan satu halaman buku milik user beserta jumlah total buku yang cocok
func (r *bookRepository) ListByUser(ctx context.Context, userID int, q BookListQuery) ([]models.Book, int, error) {
	where := "WHERE user_id = $1"
	args := []interface{}{userID}

	if q.Search != "" {
		// Escape karakter khusus LIKE agar '%' dan '_' dari user dicari apa adanya
		escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(q.Search)
		where += " AND (title ILIKE $2 OR author ILIKE $2)"
		args = append(args, "%"+escaped+"%")
	}

	var total int
	if err := r.db.QueryRow(ctx, "SELECT COUNT(*) FROM books "+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	column, ok := SortableColumns[q.SortColumn]
	if !ok {
		column = SortableColumns["created_at"]
	}
	direction := "ASC"
	if q.Desc {
		direction = "DESC"
	}

	query := fmt.Sprintf("SELECT %s FROM books %s ORDER BY %s %s, id %s LIMIT %d OFFSET %d",
		bookColumns, where, column, direction, direction, q.Limit, q.Offset)
	books, err := r.queryBooks(ctx, query, args...)
	return books, total, err
}

func (r *bookRepository) GetByID(ctx context.Context, id int) (models.Book, error) {
	b, err := scanBook(r.db.QueryRow(ctx, `SELECT `+bookColumns+` FROM books WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return b, ErrBookNotFound
	}
	return b, err
}

// Dipakai saat startup untuk membuat ulang sampul yang belum sempat dibuat worker
func (r *bookRepository) GetWithoutThumbnail(ctx context.Context) ([]models.Book, error) {
	return r.queryBooks(ctx, `SELECT `+bookColumns+` FROM books WHERE thumbnail_path = '' AND pdf_path <> '' ORDER BY id`)
}

func (r *bookRepository) Update(ctx context.Context, book models.Book) error {
	query := `UPDATE books SET title = $1, author = $2, published_year = $3 WHERE id = $4`
	_, err := r.db.Exec(ctx, query, book.Title, book.Author, book.PublishedYear, book.ID)
	return err
}

// finished = true akan mengisi finished_at (hanya sekali, saat pertama kali selesai)
func (r *bookRepository) UpdateProgress(ctx context.Context, id int, page int, finished bool) error {
	query := `UPDATE books SET last_read_page = $1,
	          finished_at = CASE WHEN $3 AND finished_at IS NULL THEN NOW() ELSE finished_at END
	          WHERE id = $2`
	_, err := r.db.Exec(ctx, query, page, id, finished)
	return err
}

func (r *bookRepository) UpdateFavorite(ctx context.Context, id int, isFavorite bool) error {
	query := `UPDATE books SET is_favorite = $1 WHERE id = $2`
	_, err := r.db.Exec(ctx, query, isFavorite, id)
	return err
}

// Khusus untuk diupdate oleh Worker Asynq
func (r *bookRepository) UpdateThumbnailPath(ctx context.Context, id int, thumbPath string) error {
	query := `UPDATE books SET thumbnail_path = $1 WHERE id = $2`
	_, err := r.db.Exec(ctx, query, thumbPath, id)
	return err
}

func (r *bookRepository) UpdateTotalPages(ctx context.Context, id int, totalPages int) error {
	query := `UPDATE books SET total_pages = $1 WHERE id = $2`
	_, err := r.db.Exec(ctx, query, totalPages, id)
	return err
}

func (r *bookRepository) Delete(ctx context.Context, id int) error {
	query := `DELETE FROM books WHERE id = $1`
	_, err := r.db.Exec(ctx, query, id)
	return err
}
