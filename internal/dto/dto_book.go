package dto

type BookRequest struct {
	Title         string `json:"title" validate:"required,min=2"`
	Author        string `json:"author" validate:"required,min=2"`
	PublishedYear int    `json:"published_year" validate:"required,min=1000,max=2026"` // Validasi tahun masuk akal
}

// Response: Untuk List (Tanpa Isi Buku)
type BookSummaryResponse struct {
	ID            int    `json:"id"`
	Title         string `json:"title"`
	Author        string `json:"author"`
	PublishedYear int    `json:"published_year"`
	ThumbnailUrl  string `json:"thumbnail_url"`
}

// Response: Untuk Detail Lengkap (Termasuk Isi Buku)
type BookDetailResponse struct {
	ID            int    `json:"id"`
	Title         string `json:"title"`
	Author        string `json:"author"`
	PublishedYear int    `json:"published_year"`
	PdfUrl        string `json:"pdf_url"` // <-- Isi buku ada di sini
	ThumbnailUrl  string `json:"thumbnail_url"`
	LastReadPage  int    `json:"last_read_page"`
}

type UpdateProgressRequest struct {
	LastReadPage int `json:"last_read_page"`
}
