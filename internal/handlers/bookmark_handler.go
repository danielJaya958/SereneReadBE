package handlers

import (
	"net/http"
	"strconv"

	"booksReader/internal/dto"
	"booksReader/internal/service"
	"booksReader/internal/utils"

	"github.com/go-chi/chi/v5"
)

type BookmarkHandler struct {
	bookmarkService service.BookmarkService
}

func NewBookmarkHandler(srv service.BookmarkService) *BookmarkHandler {
	return &BookmarkHandler{bookmarkService: srv}
}

// List godoc
// @Summary      Ambil semua bookmark satu buku
// @Description  Urut berdasarkan halaman. Daftar ini adalah acuan sinkronisasi antar perangkat. Buku milik user lain mengembalikan 404.
// @Tags         Bookmarks
// @Security     BearerAuth
// @Produce      json
// @Param        id   path      int  true  "Book ID"
// @Success      200  {array}   dto.BookmarkResponse
// @Failure      404  {object}  map[string]interface{}
// @Router       /books/{id}/bookmarks [get]
func (h *BookmarkHandler) List(w http.ResponseWriter, r *http.Request) {
	bookID, ok := getIDFromURL(w, r)
	if !ok {
		return
	}

	bookmarks, err := h.bookmarkService.List(r.Context(), bookID, getUserID(r))
	if err != nil {
		sendServiceError(w, err)
		return
	}

	utils.SendJSON(w, http.StatusOK, "Berhasil mengambil bookmark", bookmarks)
}

// Create godoc
// @Summary      Tambah bookmark
// @Description  Jika halaman itu sudah di-bookmark, catatannya diganti dan status 200 (bukan 201). Aman dikirim ulang.
// @Tags         Bookmarks
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        id    path      int                  true  "Book ID"
// @Param        body  body      dto.BookmarkRequest  true  "Halaman dan catatan (opsional, maks 500 karakter)"
// @Success      200   {object}  dto.BookmarkResponse
// @Success      201   {object}  dto.BookmarkResponse
// @Failure      400   {object}  map[string]interface{}
// @Failure      403   {object}  map[string]interface{}
// @Failure      404   {object}  map[string]interface{}
// @Router       /books/{id}/bookmarks [post]
func (h *BookmarkHandler) Create(w http.ResponseWriter, r *http.Request) {
	bookID, ok := getIDFromURL(w, r)
	if !ok {
		return
	}

	var req dto.BookmarkRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	bookmark, created, err := h.bookmarkService.Save(r.Context(), bookID, req, getUserID(r))
	if err != nil {
		sendServiceError(w, err)
		return
	}

	if created {
		utils.SendJSON(w, http.StatusCreated, "Bookmark ditambahkan", bookmark)
		return
	}
	utils.SendJSON(w, http.StatusOK, "Bookmark diperbarui", bookmark)
}

// Delete godoc
// @Summary      Hapus bookmark
// @Tags         Bookmarks
// @Security     BearerAuth
// @Produce      json
// @Param        id          path      int  true  "Book ID"
// @Param        bookmarkId  path      int  true  "Bookmark ID"
// @Success      200  {object}  map[string]interface{}
// @Failure      403  {object}  map[string]interface{}
// @Failure      404  {object}  map[string]interface{}
// @Router       /books/{id}/bookmarks/{bookmarkId} [delete]
func (h *BookmarkHandler) Delete(w http.ResponseWriter, r *http.Request) {
	bookID, ok := getIDFromURL(w, r)
	if !ok {
		return
	}

	bookmarkID, err := strconv.Atoi(chi.URLParam(r, "bookmarkId"))
	if err != nil {
		utils.SendError(w, http.StatusBadRequest, "ID bookmark tidak valid")
		return
	}

	if err := h.bookmarkService.Delete(r.Context(), bookID, bookmarkID, getUserID(r)); err != nil {
		sendServiceError(w, err)
		return
	}

	utils.SendJSON(w, http.StatusOK, "Bookmark dihapus", nil)
}
