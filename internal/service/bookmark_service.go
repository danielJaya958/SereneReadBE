package service

import (
	"context"
	"strings"
	"unicode/utf8"

	"booksReader/internal/dto"
	"booksReader/internal/models"
	"booksReader/internal/repository"
)

const maxNoteLength = 500

type BookmarkService interface {
	List(ctx context.Context, bookID int, userID int) ([]dto.BookmarkResponse, error)
	// Save mengembalikan created = true jika bookmark baru (201), false jika catatan diganti (200)
	Save(ctx context.Context, bookID int, req dto.BookmarkRequest, userID int) (dto.BookmarkResponse, bool, error)
	Delete(ctx context.Context, bookID int, bookmarkID int, userID int) error
}

type bookmarkService struct {
	repo     repository.BookmarkRepository
	bookRepo repository.BookRepository
}

func NewBookmarkService(repo repository.BookmarkRepository, bookRepo repository.BookRepository) BookmarkService {
	return &bookmarkService{repo: repo, bookRepo: bookRepo}
}

func toBookmarkResponse(bm models.Bookmark) dto.BookmarkResponse {
	return dto.BookmarkResponse{
		ID:        bm.ID,
		BookID:    bm.BookID,
		Page:      bm.Page,
		Note:      bm.Note,
		CreatedAt: bm.CreatedAt,
		UpdatedAt: bm.UpdatedAt,
	}
}

func (s *bookmarkService) List(ctx context.Context, bookID int, userID int) ([]dto.BookmarkResponse, error) {
	if _, err := findOwnedBook(ctx, s.bookRepo, bookID, userID); err != nil {
		return nil, err
	}

	bookmarks, err := s.repo.ListByBook(ctx, userID, bookID)
	if err != nil {
		return nil, err
	}

	results := make([]dto.BookmarkResponse, 0, len(bookmarks))
	for _, bm := range bookmarks {
		results = append(results, toBookmarkResponse(bm))
	}
	return results, nil
}

func (s *bookmarkService) Save(ctx context.Context, bookID int, req dto.BookmarkRequest, userID int) (dto.BookmarkResponse, bool, error) {
	book, err := findBookForWrite(ctx, s.bookRepo, bookID, userID)
	if err != nil {
		return dto.BookmarkResponse{}, false, err
	}

	if req.Page < 1 {
		return dto.BookmarkResponse{}, false, newValidationError("Nomor halaman tidak valid")
	}
	if book.TotalPages > 0 && req.Page > book.TotalPages {
		return dto.BookmarkResponse{}, false, newValidationError("Nomor halaman melebihi total halaman (%d)", book.TotalPages)
	}

	// Catatan kosong disimpan sebagai NULL
	var note *string
	if req.Note != nil {
		trimmed := strings.TrimSpace(*req.Note)
		if utf8.RuneCountInString(trimmed) > maxNoteLength {
			return dto.BookmarkResponse{}, false, newValidationError("Catatan maksimal %d karakter", maxNoteLength)
		}
		if trimmed != "" {
			note = &trimmed
		}
	}

	saved, created, err := s.repo.Upsert(ctx, models.Bookmark{
		UserID: userID,
		BookID: bookID,
		Page:   req.Page,
		Note:   note,
	})
	if err != nil {
		return dto.BookmarkResponse{}, false, err
	}
	return toBookmarkResponse(saved), created, nil
}

func (s *bookmarkService) Delete(ctx context.Context, bookID int, bookmarkID int, userID int) error {
	if _, err := findBookForWrite(ctx, s.bookRepo, bookID, userID); err != nil {
		return err
	}
	return s.repo.Delete(ctx, bookmarkID, userID, bookID)
}
