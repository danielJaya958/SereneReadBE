package repository

import (
	"context"
	"errors"

	"booksReader/internal/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrBookmarkNotFound = errors.New("bookmark tidak ditemukan")

type BookmarkRepository interface {
	ListByBook(ctx context.Context, userID int, bookID int) ([]models.Bookmark, error)
	// Upsert mengembalikan inserted = true jika bookmark baru dibuat, false jika catatan bookmark lama diganti
	Upsert(ctx context.Context, bm models.Bookmark) (models.Bookmark, bool, error)
	Delete(ctx context.Context, id int, userID int, bookID int) error
}

type bookmarkRepository struct {
	db *pgxpool.Pool
}

func NewBookmarkRepository(db *pgxpool.Pool) BookmarkRepository {
	return &bookmarkRepository{db: db}
}

func scanBookmark(row pgx.Row, extra ...interface{}) (models.Bookmark, error) {
	var bm models.Bookmark
	dest := []interface{}{&bm.ID, &bm.UserID, &bm.BookID, &bm.Page, &bm.Note, &bm.CreatedAt, &bm.UpdatedAt}
	err := row.Scan(append(dest, extra...)...)
	return bm, err
}

func (r *bookmarkRepository) ListByBook(ctx context.Context, userID int, bookID int) ([]models.Bookmark, error) {
	query := `SELECT id, user_id, book_id, page, note, created_at, updated_at FROM bookmarks WHERE user_id = $1 AND book_id = $2 ORDER BY page`
	rows, err := r.db.Query(ctx, query, userID, bookID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	bookmarks := []models.Bookmark{}
	for rows.Next() {
		bm, err := scanBookmark(rows)
		if err != nil {
			return nil, err
		}
		bookmarks = append(bookmarks, bm)
	}
	return bookmarks, rows.Err()
}

func (r *bookmarkRepository) Upsert(ctx context.Context, bm models.Bookmark) (models.Bookmark, bool, error) {
	// (xmax = 0) bernilai true hanya untuk baris yang baru di-INSERT, false jika kena DO UPDATE
	query := `INSERT INTO bookmarks (user_id, book_id, page, note)
	          VALUES ($1, $2, $3, $4)
	          ON CONFLICT (user_id, book_id, page)
	          DO UPDATE SET note = EXCLUDED.note, updated_at = CURRENT_TIMESTAMP
	          RETURNING id, user_id, book_id, page, note, created_at, updated_at, (xmax = 0) AS inserted`

	var inserted bool
	saved, err := scanBookmark(r.db.QueryRow(ctx, query, bm.UserID, bm.BookID, bm.Page, bm.Note), &inserted)
	return saved, inserted, err
}

func (r *bookmarkRepository) Delete(ctx context.Context, id int, userID int, bookID int) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM bookmarks WHERE id = $1 AND user_id = $2 AND book_id = $3`, id, userID, bookID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrBookmarkNotFound
	}
	return nil
}
