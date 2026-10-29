package repository

import (
	"context"
	"errors"

	"booksReader/internal/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type BookRepository interface {
	Create(ctx context.Context, book models.Book) (int, error)
	GetAll(ctx context.Context) ([]models.Book, error)
	GetByID(ctx context.Context, id int) (models.Book, error)
	Update(ctx context.Context, book models.Book) error
	UpdateProgress(ctx context.Context, id int, page int) error
	UpdateThumbnailPath(ctx context.Context, id int, thumbPath string) error
	Delete(ctx context.Context, id int) error
}

type bookRepository struct {
	db *pgxpool.Pool
}

func NewBookRepository(db *pgxpool.Pool) BookRepository {
	return &bookRepository{db: db}
}

func (r *bookRepository) Create(ctx context.Context, book models.Book) (int, error) {
	// UBAH QUERY: Tambahkan 'RETURNING id' di akhir
	query := `INSERT INTO books (user_id, title, author, published_year, pdf_path, thumbnail_path) 
	          VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`

	var insertedID int
	err := r.db.QueryRow(ctx, query, book.UserID, book.Title, book.Author, book.PublishedYear, book.PdfPath, book.ThumbnailPath).Scan(&insertedID)
	return insertedID, err
}

func (r *bookRepository) GetAll(ctx context.Context) ([]models.Book, error) {
	query := `SELECT id, user_id, title, author, published_year, thumbnail_path FROM books ORDER BY created_at DESC`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var books []models.Book
	for rows.Next() {
		var b models.Book
		if err := rows.Scan(&b.ID, &b.UserID, &b.Title, &b.Author, &b.PublishedYear, &b.ThumbnailPath); err != nil {
			return nil, err
		}
		books = append(books, b)
	}
	return books, nil
}

func (r *bookRepository) GetByID(ctx context.Context, id int) (models.Book, error) {
	query := `SELECT id, user_id, title, author, published_year, pdf_path, thumbnail_path, last_read_page FROM books WHERE id = $1`
	var b models.Book
	err := r.db.QueryRow(ctx, query, id).Scan(&b.ID, &b.UserID, &b.Title, &b.Author, &b.PublishedYear, &b.PdfPath, &b.ThumbnailPath, &b.LastReadPage)
	if errors.Is(err, pgx.ErrNoRows) {
		return b, errors.New("buku tidak ditemukan")
	}
	return b, err
}

func (r *bookRepository) Update(ctx context.Context, book models.Book) error {
	query := `UPDATE books SET title = $1, author = $2, published_year = $3, pdf_path = $4 WHERE id = $5`
	_, err := r.db.Exec(ctx, query, book.Title, book.Author, book.PublishedYear, book.PdfPath, book.ID)
	return err
}

func (r *bookRepository) UpdateProgress(ctx context.Context, id int, page int) error {
	query := `UPDATE books SET last_read_page = $1 WHERE id = $2`
	_, err := r.db.Exec(ctx, query, page, id)
	return err
}

// Khusus untuk diupdate oleh Worker Asynq
func (r *bookRepository) UpdateThumbnailPath(ctx context.Context, id int, thumbPath string) error {
	query := `UPDATE books SET thumbnail_path = $1 WHERE id = $2`
	_, err := r.db.Exec(ctx, query, thumbPath, id)
	return err
}

func (r *bookRepository) Delete(ctx context.Context, id int) error {
	query := `DELETE FROM books WHERE id = $1`
	_, err := r.db.Exec(ctx, query, id)
	return err
}
