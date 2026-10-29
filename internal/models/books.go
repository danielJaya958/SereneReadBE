package models

import "time"

type Book struct {
	ID            int        `json:"id" db:"id"`
	UserID        int        `json:"user_id" db:"user_id"` // Tambahan baru
	Title         string     `json:"title" db:"title"`
	Author        string     `json:"author" db:"author"`
	PublishedYear int        `json:"published_year" db:"published_year"`
	PdfPath       string     `json:"pdf_path" db:"pdf_path"`
	ThumbnailPath string     `json:"thumbnail_path" db:"thumbnail_path"`
	LastReadPage  int        `json:"last_read_page" db:"last_read_page"`
	TotalPages    int        `json:"total_pages" db:"total_pages"`
	IsFavorite    bool       `json:"is_favorite" db:"is_favorite"`
	FinishedAt    *time.Time `json:"finished_at" db:"finished_at"` // nil = belum selesai dibaca
	CreatedAt     time.Time  `json:"created_at" db:"created_at"`
}
