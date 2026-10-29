package dto

import "time"

type BookmarkRequest struct {
	Page int     `json:"page"`
	Note *string `json:"note"` // Opsional
}

type BookmarkResponse struct {
	ID        int       `json:"id"`
	BookID    int       `json:"book_id"`
	Page      int       `json:"page"`
	Note      *string   `json:"note"` // null = tanpa catatan
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
