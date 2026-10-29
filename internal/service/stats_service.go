package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"booksReader/internal/dto"
	"booksReader/internal/models"
	"booksReader/internal/repository"

	"github.com/google/uuid"
)

const (
	maxSessionSeconds = 12 * 60 * 60 // Sama dengan CHECK ck_session_length di database
	maxSessionFuture  = 5 * time.Minute
	maxLocalDateDrift = 24 * time.Hour // Toleransi zona waktu antara local_date dan started_at
	maxStatsRangeDays = 366
	defaultTopBooks   = 5
	maxTopBooks       = 50
	dateLayout        = "2006-01-02"
)

type StatsService interface {
	SaveSessions(ctx context.Context, sessions []json.RawMessage, userID int) (dto.ReadingSessionsResult, error)
	Summary(ctx context.Context, today string, userID int) (dto.StatsSummaryResponse, error)
	Daily(ctx context.Context, from string, to string, userID int) ([]dto.DailyStat, error)
	Books(ctx context.Context, from string, to string, limit int, userID int) ([]dto.BookStat, error)
}

type statsService struct {
	repo     repository.StatsRepository
	bookRepo repository.BookRepository
}

func NewStatsService(repo repository.StatsRepository, bookRepo repository.BookRepository) StatsService {
	return &statsService{repo: repo, bookRepo: bookRepo}
}

func parseDate(name string, value string) (time.Time, error) {
	t, err := time.Parse(dateLayout, value)
	if err != nil {
		return time.Time{}, newValidationError("Parameter '%s' harus berformat YYYY-MM-DD", name)
	}
	return t, nil
}

// parseRange memvalidasi from/to (keduanya wajib) dan panjang rentangnya
func parseRange(from string, to string) (time.Time, time.Time, error) {
	fromDate, err := parseDate("from", from)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	toDate, err := parseDate("to", to)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}

	if fromDate.After(toDate) {
		return time.Time{}, time.Time{}, newValidationError("Parameter 'from' tidak boleh setelah 'to'")
	}
	if days := int(toDate.Sub(fromDate).Hours()/24) + 1; days > maxStatsRangeDays {
		return time.Time{}, time.Time{}, newValidationError("Rentang tanggal maksimal %d hari", maxStatsRangeDays)
	}
	return fromDate, toDate, nil
}

func secondsToMinutes(seconds int) int {
	return seconds / 60
}

// SaveSessions memvalidasi setiap sesi secara terpisah: sesi yang valid tetap disimpan walau ada yang ditolak
func (s *statsService) SaveSessions(ctx context.Context, sessions []json.RawMessage, userID int) (dto.ReadingSessionsResult, error) {
	result := dto.ReadingSessionsResult{Rejected: []dto.RejectedSession{}}
	books := map[int]models.Book{} // Cache buku agar tidak query berulang untuk buku yang sama
	valid := make([]models.ReadingSession, 0, len(sessions))

	for i, raw := range sessions {
		var in dto.ReadingSessionInput
		if err := json.Unmarshal(raw, &in); err != nil {
			result.Rejected = append(result.Rejected, dto.RejectedSession{Index: i, Message: "Format sesi tidak valid"})
			continue
		}

		session, reason, err := s.validateSession(ctx, in, userID, books)
		if err != nil {
			return dto.ReadingSessionsResult{}, err
		}
		if reason != nil {
			rejected := dto.RejectedSession{Index: i, Message: reason.Error()}
			if in.ClientSessionID != nil {
				rejected.ClientSessionID = *in.ClientSessionID
			}
			result.Rejected = append(result.Rejected, rejected)
			continue
		}
		valid = append(valid, session)
	}

	accepted, duplicates, err := s.repo.InsertSessions(ctx, valid)
	if err != nil {
		return dto.ReadingSessionsResult{}, err
	}
	result.Accepted = accepted
	result.Duplicates = duplicates
	return result, nil
}

// validateSession mengembalikan reason != nil jika sesi ditolak (pesan untuk HP),
// dan err != nil hanya untuk kesalahan server (misalnya database mati).
func (s *statsService) validateSession(ctx context.Context, in dto.ReadingSessionInput, userID int, books map[int]models.Book) (models.ReadingSession, error, error) {
	if in.ClientSessionID == nil || in.BookID == nil || in.StartedAt == nil || in.EndedAt == nil || in.LocalDate == nil ||
		in.StartPage == nil || in.EndPage == nil || in.PagesRead == nil || in.DurationSeconds == nil {
		return models.ReadingSession{}, errors.New("Semua field sesi wajib diisi"), nil
	}

	clientID, err := uuid.Parse(*in.ClientSessionID)
	if err != nil {
		return models.ReadingSession{}, errors.New("client_session_id harus berupa UUID"), nil
	}

	startedAt, err := time.Parse(time.RFC3339, *in.StartedAt)
	if err != nil {
		return models.ReadingSession{}, errors.New("started_at harus berformat RFC 3339, contoh 2026-10-24T20:15:00+07:00"), nil
	}
	endedAt, err := time.Parse(time.RFC3339, *in.EndedAt)
	if err != nil {
		return models.ReadingSession{}, errors.New("ended_at harus berformat RFC 3339, contoh 2026-10-24T20:42:30+07:00"), nil
	}
	if !endedAt.After(startedAt) {
		return models.ReadingSession{}, errors.New("ended_at harus setelah started_at"), nil
	}
	if startedAt.After(time.Now().Add(maxSessionFuture)) {
		return models.ReadingSession{}, errors.New("started_at tidak boleh di masa depan"), nil
	}

	duration := *in.DurationSeconds
	if duration < 1 || duration > maxSessionSeconds {
		return models.ReadingSession{}, fmt.Errorf("duration_seconds harus di antara 1 dan %d", maxSessionSeconds), nil
	}
	// duration_seconds boleh lebih kecil (waktu jeda tidak dihitung), tapi tidak boleh lebih besar
	if float64(duration) > math.Ceil(endedAt.Sub(startedAt).Seconds()) {
		return models.ReadingSession{}, errors.New("duration_seconds melebihi selisih ended_at - started_at"), nil
	}

	localDate, err := time.Parse(dateLayout, *in.LocalDate)
	if err != nil {
		return models.ReadingSession{}, errors.New("local_date harus berformat YYYY-MM-DD"), nil
	}
	startedDate := time.Date(startedAt.Year(), startedAt.Month(), startedAt.Day(), 0, 0, 0, 0, time.UTC)
	if drift := localDate.Sub(startedDate); drift < -maxLocalDateDrift || drift > maxLocalDateDrift {
		return models.ReadingSession{}, errors.New("local_date terlalu jauh dari tanggal started_at"), nil
	}

	book, reason, err := s.ownedBook(ctx, *in.BookID, userID, books)
	if err != nil || reason != nil {
		return models.ReadingSession{}, reason, err
	}

	if *in.StartPage < 1 || *in.EndPage < 1 || *in.PagesRead < 0 {
		return models.ReadingSession{}, errors.New("Nomor halaman tidak valid"), nil
	}
	if total := book.TotalPages; total > 0 && (*in.StartPage > total || *in.EndPage > total || *in.PagesRead > total) {
		return models.ReadingSession{}, fmt.Errorf("Nomor halaman melebihi total halaman (%d)", total), nil
	}

	return models.ReadingSession{
		UserID:          userID,
		BookID:          book.ID,
		ClientSessionID: clientID.String(),
		StartedAt:       startedAt,
		EndedAt:         endedAt,
		LocalDate:       localDate,
		StartPage:       *in.StartPage,
		EndPage:         *in.EndPage,
		PagesRead:       *in.PagesRead,
		DurationSeconds: duration,
	}, nil, nil
}

// ownedBook mengambil buku dari cache per request; buku milik user lain dianggap tidak ada
func (s *statsService) ownedBook(ctx context.Context, bookID int, userID int, books map[int]models.Book) (models.Book, error, error) {
	if b, ok := books[bookID]; ok {
		return b, nil, nil
	}

	b, err := findOwnedBook(ctx, s.bookRepo, bookID, userID)
	if errors.Is(err, ErrNotFound) || errors.Is(err, repository.ErrBookNotFound) {
		return models.Book{}, ErrNotFound, nil
	}
	if err != nil {
		return models.Book{}, nil, err
	}

	books[bookID] = b
	return b, nil, nil
}

func (s *statsService) Summary(ctx context.Context, today string, userID int) (dto.StatsSummaryResponse, error) {
	if today == "" {
		return dto.StatsSummaryResponse{}, newValidationError("Parameter 'today' wajib diisi (YYYY-MM-DD)")
	}
	todayDate, err := parseDate("today", today)
	if err != nil {
		return dto.StatsSummaryResponse{}, err
	}

	// Minggu dihitung Senin-Minggu
	daysSinceMonday := (int(todayDate.Weekday()) + 6) % 7
	weekStart := todayDate.AddDate(0, 0, -daysSinceMonday)
	weekEnd := weekStart.AddDate(0, 0, 6)
	yearStart := time.Date(todayDate.Year(), 1, 1, 0, 0, 0, 0, time.UTC)

	todayTotals, err := s.repo.Totals(ctx, userID, &todayDate, &todayDate)
	if err != nil {
		return dto.StatsSummaryResponse{}, err
	}
	weekTotals, err := s.repo.Totals(ctx, userID, &weekStart, &weekEnd)
	if err != nil {
		return dto.StatsSummaryResponse{}, err
	}
	allTotals, err := s.repo.Totals(ctx, userID, nil, nil)
	if err != nil {
		return dto.StatsSummaryResponse{}, err
	}
	currentStreak, longestStreak, err := s.repo.Streaks(ctx, userID, todayDate)
	if err != nil {
		return dto.StatsSummaryResponse{}, err
	}
	finishedThisYear, finishedAllTime, err := s.repo.BooksFinished(ctx, userID, yearStart)
	if err != nil {
		return dto.StatsSummaryResponse{}, err
	}
	lastRead, err := s.repo.LastReadDate(ctx, userID)
	if err != nil {
		return dto.StatsSummaryResponse{}, err
	}

	resp := dto.StatsSummaryResponse{
		Today: dto.StatsPeriod{
			Minutes: secondsToMinutes(todayTotals.Seconds),
			Pages:   todayTotals.Pages,
		},
		ThisWeek: dto.StatsWeek{
			Minutes:    secondsToMinutes(weekTotals.Seconds),
			Pages:      weekTotals.Pages,
			ActiveDays: weekTotals.ActiveDays,
		},
		AllTime: dto.StatsAllTime{
			Minutes:    secondsToMinutes(allTotals.Seconds),
			Pages:      allTotals.Pages,
			ActiveDays: allTotals.ActiveDays,
			Sessions:   allTotals.Sessions,
		},
		CurrentStreakDays: currentStreak,
		LongestStreakDays: longestStreak,
		BooksFinished: dto.BooksFinishedStats{
			ThisYear: finishedThisYear,
			AllTime:  finishedAllTime,
		},
	}
	if lastRead != nil {
		formatted := lastRead.Format(dateLayout)
		resp.LastReadDate = &formatted
	}
	return resp, nil
}

func (s *statsService) Daily(ctx context.Context, from string, to string, userID int) ([]dto.DailyStat, error) {
	fromDate, toDate, err := parseRange(from, to)
	if err != nil {
		return nil, err
	}

	days, err := s.repo.Daily(ctx, userID, fromDate, toDate)
	if err != nil {
		return nil, err
	}

	results := make([]dto.DailyStat, 0, len(days))
	for _, d := range days {
		results = append(results, dto.DailyStat{
			Date:     d.Date.Format(dateLayout),
			Minutes:  secondsToMinutes(d.Seconds),
			Pages:    d.Pages,
			Sessions: d.Sessions,
		})
	}
	return results, nil
}

// Books: from/to opsional (tanpa keduanya = semua waktu)
func (s *statsService) Books(ctx context.Context, from string, to string, limit int, userID int) ([]dto.BookStat, error) {
	var fromPtr, toPtr *time.Time
	if from != "" || to != "" {
		fromDate, toDate, err := parseRange(from, to)
		if err != nil {
			return nil, err
		}
		fromPtr, toPtr = &fromDate, &toDate
	}

	if limit == 0 {
		limit = defaultTopBooks
	}
	if limit < 1 || limit > maxTopBooks {
		return nil, newValidationError("Parameter 'limit' harus di antara 1 dan %d", maxTopBooks)
	}

	books, err := s.repo.TopBooks(ctx, userID, fromPtr, toPtr, limit)
	if err != nil {
		return nil, err
	}

	results := make([]dto.BookStat, 0, len(books))
	for _, b := range books {
		results = append(results, dto.BookStat{
			BookID:       b.BookID,
			Title:        b.Title,
			ThumbnailUrl: thumbnailURL(b.ThumbnailPath),
			Minutes:      secondsToMinutes(b.Seconds),
			Pages:        b.Pages,
			Sessions:     b.Sessions,
			LastReadDate: b.LastReadDate.Format(dateLayout),
		})
	}
	return results, nil
}
