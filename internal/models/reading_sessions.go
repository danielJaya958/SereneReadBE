package models

import "time"

type ReadingSession struct {
	ID              int64     `json:"id" db:"id"`
	UserID          int       `json:"user_id" db:"user_id"`
	BookID          int       `json:"book_id" db:"book_id"`
	ClientSessionID string    `json:"client_session_id" db:"client_session_id"`
	StartedAt       time.Time `json:"started_at" db:"started_at"`
	EndedAt         time.Time `json:"ended_at" db:"ended_at"`
	LocalDate       time.Time `json:"local_date" db:"local_date"`
	StartPage       int       `json:"start_page" db:"start_page"`
	EndPage         int       `json:"end_page" db:"end_page"`
	PagesRead       int       `json:"pages_read" db:"pages_read"`
	DurationSeconds int       `json:"duration_seconds" db:"duration_seconds"`
	CreatedAt       time.Time `json:"created_at" db:"created_at"`
}
