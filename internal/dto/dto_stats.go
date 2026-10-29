package dto

import "encoding/json"

// Request: POST /reading-sessions
// Setiap sesi disimpan mentah agar bisa divalidasi satu per satu.
type ReadingSessionsRequest struct {
	Sessions []json.RawMessage `json:"sessions"`
}

// Satu sesi dari HP. Semua field pointer agar bisa dibedakan antara "tidak dikirim" dan nilai 0.
type ReadingSessionInput struct {
	ClientSessionID *string `json:"client_session_id"`
	BookID          *int    `json:"book_id"`
	StartedAt       *string `json:"started_at"`
	EndedAt         *string `json:"ended_at"`
	LocalDate       *string `json:"local_date"`
	StartPage       *int    `json:"start_page"`
	EndPage         *int    `json:"end_page"`
	PagesRead       *int    `json:"pages_read"`
	DurationSeconds *int    `json:"duration_seconds"`
}

type RejectedSession struct {
	Index           int    `json:"index"`
	ClientSessionID string `json:"client_session_id"`
	Message         string `json:"message"`
}

type ReadingSessionsResult struct {
	Accepted   int               `json:"accepted"`
	Duplicates int               `json:"duplicates"`
	Rejected   []RejectedSession `json:"rejected"`
}

type StatsPeriod struct {
	Minutes int `json:"minutes"`
	Pages   int `json:"pages"`
}

type StatsWeek struct {
	Minutes    int `json:"minutes"`
	Pages      int `json:"pages"`
	ActiveDays int `json:"active_days"`
}

type StatsAllTime struct {
	Minutes    int `json:"minutes"`
	Pages      int `json:"pages"`
	ActiveDays int `json:"active_days"`
	Sessions   int `json:"sessions"`
}

type BooksFinishedStats struct {
	ThisYear int `json:"this_year"`
	AllTime  int `json:"all_time"`
}

type StatsSummaryResponse struct {
	Today             StatsPeriod        `json:"today"`
	ThisWeek          StatsWeek          `json:"this_week"`
	AllTime           StatsAllTime       `json:"all_time"`
	CurrentStreakDays int                `json:"current_streak_days"`
	LongestStreakDays int                `json:"longest_streak_days"`
	BooksFinished     BooksFinishedStats `json:"books_finished"`
	LastReadDate      *string            `json:"last_read_date"` // null jika belum pernah membaca
}

type DailyStat struct {
	Date     string `json:"date"`
	Minutes  int    `json:"minutes"`
	Pages    int    `json:"pages"`
	Sessions int    `json:"sessions"`
}

type BookStat struct {
	BookID       int    `json:"book_id"`
	Title        string `json:"title"`
	ThumbnailUrl string `json:"thumbnail_url"`
	Minutes      int    `json:"minutes"`
	Pages        int    `json:"pages"`
	Sessions     int    `json:"sessions"`
	LastReadDate string `json:"last_read_date"`
}
