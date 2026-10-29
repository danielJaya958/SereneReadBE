package repository

import (
	"context"
	"time"

	"booksReader/internal/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PeriodTotals adalah total membaca dalam satu rentang tanggal
type PeriodTotals struct {
	Pages      int
	Seconds    int
	ActiveDays int // Hari dengan minimal 1 halaman dibaca
	Sessions   int
}

// DailyTotals adalah total membaca untuk satu tanggal lokal
type DailyTotals struct {
	Date     time.Time
	Pages    int
	Seconds  int
	Sessions int
}

// BookTotals adalah total membaca untuk satu buku
type BookTotals struct {
	BookID        int
	Title         string
	ThumbnailPath string
	Pages         int
	Seconds       int
	Sessions      int
	LastReadDate  time.Time
}

type StatsRepository interface {
	// InsertSessions mengembalikan jumlah sesi yang tersimpan dan jumlah duplikat (client_session_id sudah ada)
	InsertSessions(ctx context.Context, sessions []models.ReadingSession) (int, int, error)
	// from/to nil = tanpa batas
	Totals(ctx context.Context, userID int, from *time.Time, to *time.Time) (PeriodTotals, error)
	// Streaks mengembalikan streak saat ini (berakhir hari ini atau kemarin) dan streak terpanjang
	Streaks(ctx context.Context, userID int, today time.Time) (int, int, error)
	LastReadDate(ctx context.Context, userID int) (*time.Time, error)
	// BooksFinished mengembalikan jumlah buku selesai sejak yearStart dan sepanjang waktu
	BooksFinished(ctx context.Context, userID int, yearStart time.Time) (int, int, error)
	Daily(ctx context.Context, userID int, from time.Time, to time.Time) ([]DailyTotals, error)
	TopBooks(ctx context.Context, userID int, from *time.Time, to *time.Time, limit int) ([]BookTotals, error)
}

type statsRepository struct {
	db *pgxpool.Pool
}

func NewStatsRepository(db *pgxpool.Pool) StatsRepository {
	return &statsRepository{db: db}
}

func (r *statsRepository) InsertSessions(ctx context.Context, sessions []models.ReadingSession) (int, int, error) {
	if len(sessions) == 0 {
		return 0, 0, nil
	}

	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback(ctx)

	query := `INSERT INTO reading_sessions
	          (user_id, book_id, client_session_id, started_at, ended_at, local_date,
	           start_page, end_page, pages_read, duration_seconds)
	          VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	          ON CONFLICT (user_id, client_session_id) DO NOTHING`

	inserted, duplicates := 0, 0
	for _, s := range sessions {
		tag, err := tx.Exec(ctx, query, s.UserID, s.BookID, s.ClientSessionID, s.StartedAt, s.EndedAt, s.LocalDate,
			s.StartPage, s.EndPage, s.PagesRead, s.DurationSeconds)
		if err != nil {
			return 0, 0, err
		}
		if tag.RowsAffected() == 1 {
			inserted++
		} else {
			duplicates++
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, 0, err
	}
	return inserted, duplicates, nil
}

func (r *statsRepository) Totals(ctx context.Context, userID int, from *time.Time, to *time.Time) (PeriodTotals, error) {
	query := `SELECT COALESCE(SUM(pages), 0)::int,
	                 COALESCE(SUM(seconds), 0)::int,
	                 COUNT(*) FILTER (WHERE pages >= 1)::int,
	                 COALESCE(SUM(sessions), 0)::int
	          FROM (
	              SELECT SUM(pages_read) AS pages, SUM(duration_seconds) AS seconds, COUNT(*) AS sessions
	              FROM reading_sessions
	              WHERE user_id = $1
	                AND ($2::date IS NULL OR local_date >= $2::date)
	                AND ($3::date IS NULL OR local_date <= $3::date)
	              GROUP BY local_date
	          ) AS per_day`

	var t PeriodTotals
	err := r.db.QueryRow(ctx, query, userID, from, to).Scan(&t.Pages, &t.Seconds, &t.ActiveDays, &t.Sessions)
	return t, err
}

func (r *statsRepository) Streaks(ctx context.Context, userID int, today time.Time) (int, int, error) {
	// Teknik "gaps and islands": tanggal berurutan dikurangi nomor urutnya menghasilkan grp yang sama
	query := `WITH active AS (
	              SELECT local_date
	              FROM reading_sessions
	              WHERE user_id = $1
	              GROUP BY local_date
	              HAVING SUM(pages_read) >= 1
	          ),
	          islands AS (
	              SELECT local_date, local_date - (ROW_NUMBER() OVER (ORDER BY local_date))::int AS grp
	              FROM active
	          ),
	          streaks AS (
	              SELECT MIN(local_date) AS start_date, MAX(local_date) AS end_date, COUNT(*) AS length
	              FROM islands
	              GROUP BY grp
	          )
	          SELECT COALESCE(MAX(length) FILTER (WHERE end_date >= $2::date - 1 AND start_date <= $2::date), 0)::int,
	                 COALESCE(MAX(length), 0)::int
	          FROM streaks`

	var current, longest int
	err := r.db.QueryRow(ctx, query, userID, today).Scan(&current, &longest)
	return current, longest, err
}

func (r *statsRepository) LastReadDate(ctx context.Context, userID int) (*time.Time, error) {
	var last *time.Time
	err := r.db.QueryRow(ctx, `SELECT MAX(local_date) FROM reading_sessions WHERE user_id = $1`, userID).Scan(&last)
	return last, err
}

func (r *statsRepository) BooksFinished(ctx context.Context, userID int, yearStart time.Time) (int, int, error) {
	query := `SELECT COUNT(*) FILTER (WHERE finished_at >= $2::date AND finished_at < ($2::date + INTERVAL '1 year'))::int,
	                 COUNT(finished_at)::int
	          FROM books
	          WHERE user_id = $1`

	var thisYear, allTime int
	err := r.db.QueryRow(ctx, query, userID, yearStart).Scan(&thisYear, &allTime)
	return thisYear, allTime, err
}

func (r *statsRepository) Daily(ctx context.Context, userID int, from time.Time, to time.Time) ([]DailyTotals, error) {
	// generate_series memastikan hari tanpa sesi tetap muncul dengan nilai 0
	query := `SELECT days.d::date,
	                 COALESCE(SUM(s.pages_read), 0)::int,
	                 COALESCE(SUM(s.duration_seconds), 0)::int,
	                 COUNT(s.id)::int
	          FROM generate_series($2::date, $3::date, INTERVAL '1 day') AS days(d)
	          LEFT JOIN reading_sessions s ON s.user_id = $1 AND s.local_date = days.d::date
	          GROUP BY days.d
	          ORDER BY days.d`

	rows, err := r.db.Query(ctx, query, userID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	days := []DailyTotals{}
	for rows.Next() {
		var d DailyTotals
		if err := rows.Scan(&d.Date, &d.Pages, &d.Seconds, &d.Sessions); err != nil {
			return nil, err
		}
		days = append(days, d)
	}
	return days, rows.Err()
}

func (r *statsRepository) TopBooks(ctx context.Context, userID int, from *time.Time, to *time.Time, limit int) ([]BookTotals, error) {
	// JOIN (bukan LEFT JOIN): sesi dari buku yang sudah dihapus (book_id NULL) tidak ikut
	query := `SELECT b.id, b.title, b.thumbnail_path,
	                 SUM(s.pages_read)::int,
	                 SUM(s.duration_seconds)::int,
	                 COUNT(*)::int,
	                 MAX(s.local_date)
	          FROM reading_sessions s
	          JOIN books b ON b.id = s.book_id
	          WHERE s.user_id = $1
	            AND ($2::date IS NULL OR s.local_date >= $2::date)
	            AND ($3::date IS NULL OR s.local_date <= $3::date)
	          GROUP BY b.id, b.title, b.thumbnail_path
	          ORDER BY SUM(s.duration_seconds) DESC, b.id
	          LIMIT $4`

	rows, err := r.db.Query(ctx, query, userID, from, to, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	books := []BookTotals{}
	for rows.Next() {
		var b BookTotals
		if err := rows.Scan(&b.BookID, &b.Title, &b.ThumbnailPath, &b.Pages, &b.Seconds, &b.Sessions, &b.LastReadDate); err != nil {
			return nil, err
		}
		books = append(books, b)
	}
	return books, rows.Err()
}
