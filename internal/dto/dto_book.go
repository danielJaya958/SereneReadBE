package dto

import "time"

// Request: PUT /books/{id} (semua field wajib, divalidasi oleh service.ValidateBookInput)
type BookRequest struct {
	Title         string `json:"title"`
	Author        string `json:"author"`
	PublishedYear int    `json:"published_year"`
}

// Request: PATCH /books/{id} (semua field opsional; nil = tidak diubah)
type UpdateBookRequest struct {
	Title         *string `json:"title"`
	Author        *string `json:"author"`
	PublishedYear *int    `json:"published_year"`
}

// Query GET /books dengan paginasi
type BookListParams struct {
	Q     string
	Sort  string
	Page  int
	Limit int
}

type PaginationMeta struct {
	Page       int  `json:"page"`
	Limit      int  `json:"limit"`
	TotalItems int  `json:"total_items"`
	HasNext    bool `json:"has_next"`
}

// Response: Untuk List (Tanpa Isi Buku)
type BookSummaryResponse struct {
	ID            int        `json:"id"`
	Title         string     `json:"title"`
	Author        string     `json:"author"`
	PublishedYear int        `json:"published_year"`
	ThumbnailUrl  string     `json:"thumbnail_url"`  // "" = sampul belum dibuat
	LastReadPage  int        `json:"last_read_page"` // 0 = belum pernah dibaca
	TotalPages    int        `json:"total_pages"`    // 0 = belum dihitung
	IsFavorite    bool       `json:"is_favorite"`
	FinishedAt    *time.Time `json:"finished_at"` // null = belum selesai dibaca
}

// Response: Untuk Detail Lengkap (Termasuk Isi Buku)
type BookDetailResponse struct {
	ID            int        `json:"id"`
	Title         string     `json:"title"`
	Author        string     `json:"author"`
	PublishedYear int        `json:"published_year"`
	PdfUrl        string     `json:"pdf_url"` // <-- Isi buku ada di sini
	ThumbnailUrl  string     `json:"thumbnail_url"`
	LastReadPage  int        `json:"last_read_page"`
	TotalPages    int        `json:"total_pages"`
	IsFavorite    bool       `json:"is_favorite"`
	FinishedAt    *time.Time `json:"finished_at"`
}

type BookContentResponse struct {
	PdfUrl       string `json:"pdf_url"`
	LastReadPage int    `json:"last_read_page"`
	TotalPages   int    `json:"total_pages"`
}

type UpdateProgressRequest struct {
	LastReadPage int `json:"last_read_page"`
}

type FavoriteRequest struct {
	IsFavorite *bool `json:"is_favorite"`
}
