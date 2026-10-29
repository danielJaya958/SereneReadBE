package models

import "time"

type Bookmark struct {
	ID        int       `json:"id" db:"id"`
	UserID    int       `json:"user_id" db:"user_id"`
	BookID    int       `json:"book_id" db:"book_id"`
	Page      int       `json:"page" db:"page"`
	Note      *string   `json:"note" db:"note"` // nil = tanpa catatan
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}
