package handlers

import (
	"encoding/json"
	"errors"
	"io" // TAMBAHAN: Untuk Copy file
	"log"
	"net/http"
	"net/url"
	"os"            // TAMBAHAN: Untuk manipulasi File/Folder
	"path/filepath" // TAMBAHAN: Untuk ekstensi file (.pdf)
	"strconv"
	"strings" // TAMBAHAN: Untuk ToLower (mengecilkan huruf)
	"time"    // TAMBAHAN: Untuk Timestamp nama file

	"booksReader/internal/dto"
	customMiddleware "booksReader/internal/middleware"
	"booksReader/internal/repository"
	"booksReader/internal/service"
	"booksReader/internal/utils"

	"github.com/go-chi/chi/v5"
)

type BookHandler struct {
	bookService service.BookService
}

func NewBookHandler(srv service.BookService) *BookHandler {
	return &BookHandler{bookService: srv}
}

// Helper untuk parse ID dari URL (contoh: /books/12). Mengirim 400 sendiri jika tidak valid.
func getIDFromURL(w http.ResponseWriter, r *http.Request) (int, bool) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		utils.SendError(w, http.StatusBadRequest, "ID tidak valid")
		return 0, false
	}
	return id, true
}

// UserID diisi oleh AuthMiddleware
func getUserID(r *http.Request) int {
	return r.Context().Value(customMiddleware.UserIDKey).(int)
}

// Helper decode body JSON. Mengirim 400 sendiri jika gagal.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst interface{}) bool {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		utils.SendError(w, http.StatusBadRequest, "Format JSON tidak valid")
		return false
	}
	return true
}

// sendServiceError memetakan error dari service ke status HTTP.
// Error yang tidak dikenal dicatat ke log dan tidak ditampilkan ke user.
func sendServiceError(w http.ResponseWriter, err error) {
	var validationErr *service.ValidationError
	switch {
	case errors.As(err, &validationErr):
		utils.SendError(w, http.StatusBadRequest, validationErr.Message)
	case errors.Is(err, service.ErrNotFound), errors.Is(err, repository.ErrBookNotFound), errors.Is(err, repository.ErrBookmarkNotFound):
		utils.SendError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, service.ErrForbidden):
		utils.SendError(w, http.StatusForbidden, err.Error())
	default:
		log.Printf("❌ Internal error: %v", err)
		utils.SendError(w, http.StatusInternalServerError, "Terjadi kesalahan pada server")
	}
}

// Create godoc
// @Summary      Upload buku PDF baru
// @Description  Mengembalikan data buku yang baru dibuat. thumbnail_url masih kosong sampai worker selesai membuat sampul.
// @Tags         Books
// @Security     BearerAuth
// @Accept       multipart/form-data
// @Produce      json
// @Param        title           formData  string  true  "Judul buku"
// @Param        author          formData  string  true  "Nama pengarang"
// @Param        published_year  formData  int     true  "Tahun terbit"
// @Param        pdf_file        formData  file    true  "File PDF (Max 75MB)"
// @Success      201  {object}  dto.BookDetailResponse
// @Failure      400  {object}  map[string]interface{}
// @Failure      500  {object}  map[string]interface{}
// @Router       /books [post]
func (h *BookHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)

	// 1. Batasi ukuran file maksimal 75 MB
	err := r.ParseMultipartForm(75 << 20)
	if err != nil {
		utils.SendError(w, http.StatusBadRequest, "Ukuran file terlalu besar (Max 75MB)")
		return
	}

	// 2. Ambil data teks dari Form
	title := r.FormValue("title")
	author := r.FormValue("author")
	year, _ := strconv.Atoi(r.FormValue("published_year"))

	// Validasi data teks sebelum menyimpan file
	if err := service.ValidateBookInput(title, author, year); err != nil {
		sendServiceError(w, err)
		return
	}

	// 3. Ambil File PDF dari Form
	file, fileHeader, err := r.FormFile("pdf_file")
	if err != nil {
		utils.SendError(w, http.StatusBadRequest, "Gagal membaca file PDF")
		return
	}
	defer file.Close()

	// 4. Validasi Ekstensi File (Harus .pdf)
	ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
	if ext != ".pdf" {
		utils.SendError(w, http.StatusBadRequest, "Format file harus PDF")
		return
	}

	// 5. Buat nama file yang unik (Mencegah nama file kembar saling timpa)
	fileName := strconv.Itoa(userID) + "_" + strconv.FormatInt(time.Now().Unix(), 10) + ext

	// Tentukan folder penyimpanan (Buat folder jika belum ada)
	uploadDir := "./uploads/pdf/"
	if _, err := os.Stat(uploadDir); os.IsNotExist(err) {
		os.MkdirAll(uploadDir, os.ModePerm)
	}

	fullPath := uploadDir + fileName

	// 6. Simpan File ke Harddisk Server
	dest, err := os.Create(fullPath)
	if err != nil {
		utils.SendError(w, http.StatusInternalServerError, "Gagal menyimpan file ke server")
		return
	}

	// Copy isi file yang diupload ke file baru di server.
	// File ditutup sebelum service membaca jumlah halamannya.
	_, err = io.Copy(dest, file)
	dest.Close()
	if err != nil {
		os.Remove(fullPath)
		utils.SendError(w, http.StatusInternalServerError, "Gagal menulis file")
		return
	}

	// 7. Simpan Info Buku ke DB & Kirim Tugas ke Worker
	book, err := h.bookService.Create(r.Context(), title, author, year, fileName, userID)
	if err != nil {
		// Rollback: Hapus PDF jika gagal simpan ke DB
		os.Remove(fullPath)
		sendServiceError(w, err)
		return
	}

	// 8. Kirim Respons Cepat ke User
	utils.SendJSON(w, http.StatusCreated, "Buku PDF berhasil diupload! Sampul sedang diproses di background.", book)
}

// GetAll godoc
// @Summary      Ambil daftar buku milik user yang login
// @Description  Tanpa parameter: semua buku (perilaku lama). Dengan salah satu dari q, sort, page, atau limit: hasil dipaginasi dan response berisi field "meta".
// @Tags         Books
// @Security     BearerAuth
// @Produce      json
// @Param        q      query     string  false  "Cari di judul atau pengarang"
// @Param        sort   query     string  false  "created_at, title, author, published_year. Awali '-' untuk menurun. Default -created_at"
// @Param        page   query     int     false  "Halaman, mulai dari 1 (default 1)"
// @Param        limit  query     int     false  "Jumlah per halaman, 1-100 (default 20)"
// @Success      200  {array}   dto.BookSummaryResponse
// @Failure      400  {object}  map[string]interface{}
// @Failure      500  {object}  map[string]interface{}
// @Router       /books [get]
func (h *BookHandler) GetAll(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	// Tanpa parameter apa pun: perilaku lama (semua buku, memakai cache)
	if !query.Has("q") && !query.Has("sort") && !query.Has("page") && !query.Has("limit") {
		books, err := h.bookService.GetAll(r.Context(), getUserID(r))
		if err != nil {
			sendServiceError(w, err)
			return
		}
		utils.SendJSON(w, http.StatusOK, "Berhasil mengambil daftar buku", books)
		return
	}

	params := dto.BookListParams{Q: query.Get("q"), Sort: query.Get("sort")}
	var ok bool
	if params.Page, ok = parseQueryInt(w, query, "page"); !ok {
		return
	}
	if params.Limit, ok = parseQueryInt(w, query, "limit"); !ok {
		return
	}

	books, meta, err := h.bookService.List(r.Context(), getUserID(r), params)
	if err != nil {
		sendServiceError(w, err)
		return
	}
	utils.SendJSONWithMeta(w, http.StatusOK, "Berhasil mengambil daftar buku", books, meta)
}

// parseQueryInt membaca query parameter angka. Kosong = 0 (service yang mengisi default).
func parseQueryInt(w http.ResponseWriter, query url.Values, name string) (int, bool) {
	raw := query.Get(name)
	if raw == "" {
		return 0, true
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		utils.SendError(w, http.StatusBadRequest, "Parameter '"+name+"' harus berupa angka")
		return 0, false
	}
	return value, true
}

// GetDetail godoc
// @Summary      Ambil detail satu buku
// @Description  Hanya pemilik buku yang bisa mengakses. Buku milik user lain mengembalikan 404.
// @Tags         Books
// @Security     BearerAuth
// @Produce      json
// @Param        id   path      int  true  "Book ID"
// @Success      200  {object}  dto.BookDetailResponse
// @Failure      404  {object}  map[string]interface{}
// @Router       /books/{id} [get]
func (h *BookHandler) GetDetail(w http.ResponseWriter, r *http.Request) {
	id, ok := getIDFromURL(w, r)
	if !ok {
		return
	}

	book, err := h.bookService.GetDetail(r.Context(), id, getUserID(r))
	if err != nil {
		sendServiceError(w, err)
		return
	}

	utils.SendJSON(w, http.StatusOK, "Berhasil mengambil detail buku", book)
}

// GetContent godoc
// @Summary      Ambil URL PDF buku beserta progres baca
// @Description  Hanya pemilik buku yang bisa mengakses. Buku milik user lain mengembalikan 404.
// @Tags         Books
// @Security     BearerAuth
// @Produce      json
// @Param        id   path      int  true  "Book ID"
// @Success      200  {object}  dto.BookContentResponse
// @Failure      404  {object}  map[string]interface{}
// @Router       /books/{id}/content [get]
func (h *BookHandler) GetContent(w http.ResponseWriter, r *http.Request) {
	id, ok := getIDFromURL(w, r)
	if !ok {
		return
	}

	content, err := h.bookService.GetContent(r.Context(), id, getUserID(r))
	if err != nil {
		sendServiceError(w, err)
		return
	}

	utils.SendJSON(w, http.StatusOK, "Berhasil membaca isi buku", content)
}

// GetFile godoc
// @Summary      Download file PDF buku
// @Description  Hanya pemilik buku yang bisa mengakses. Mendukung Range request untuk PDF viewer.
// @Tags         Books
// @Security     BearerAuth
// @Produce      application/pdf
// @Param        id   path      int  true  "Book ID"
// @Success      200  {file}    file
// @Failure      404  {object}  map[string]interface{}
// @Router       /books/{id}/file [get]
func (h *BookHandler) GetFile(w http.ResponseWriter, r *http.Request) {
	id, ok := getIDFromURL(w, r)
	if !ok {
		return
	}

	path, err := h.bookService.GetPdfFilePath(r.Context(), id, getUserID(r))
	if err != nil {
		sendServiceError(w, err)
		return
	}

	if _, err := os.Stat(path); err != nil {
		utils.SendError(w, http.StatusNotFound, "File PDF tidak ditemukan")
		return
	}

	// ServeFile otomatis menangani header Range (PDF viewer bisa memuat per bagian)
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Cache-Control", "private, no-store")
	http.ServeFile(w, r, path)
}

// Update godoc
// @Summary      Ganti semua metadata buku (judul, author, tahun)
// @Description  Semua field wajib. Untuk mengubah sebagian field, gunakan PATCH.
// @Tags         Books
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        id    path      int              true  "Book ID"
// @Param        body  body      dto.BookRequest  true  "Data buku yang diupdate"
// @Success      200   {object}  dto.BookDetailResponse
// @Failure      400   {object}  map[string]interface{}
// @Failure      403   {object}  map[string]interface{}
// @Failure      404   {object}  map[string]interface{}
// @Router       /books/{id} [put]
// Update ini hanya untuk mengupdate metadata teks (Judul, Author, Tahun), bukan ganti file PDF-nya.
func (h *BookHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := getIDFromURL(w, r)
	if !ok {
		return
	}

	var req dto.BookRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	// PUT = semua field diganti, jadi semua dikirim ke service
	update := dto.UpdateBookRequest{Title: &req.Title, Author: &req.Author, PublishedYear: &req.PublishedYear}
	book, err := h.bookService.Update(r.Context(), id, update, getUserID(r))
	if err != nil {
		sendServiceError(w, err)
		return
	}

	utils.SendJSON(w, http.StatusOK, "Metadata buku berhasil diupdate", book)
}

// Patch godoc
// @Summary      Edit sebagian info buku
// @Description  Semua field opsional; field yang tidak dikirim tidak diubah. Hanya pemilik buku (selain itu 403).
// @Tags         Books
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        id    path      int                    true  "Book ID"
// @Param        body  body      dto.UpdateBookRequest  true  "Field yang diubah"
// @Success      200   {object}  dto.BookDetailResponse
// @Failure      400   {object}  map[string]interface{}
// @Failure      403   {object}  map[string]interface{}
// @Failure      404   {object}  map[string]interface{}
// @Router       /books/{id} [patch]
func (h *BookHandler) Patch(w http.ResponseWriter, r *http.Request) {
	id, ok := getIDFromURL(w, r)
	if !ok {
		return
	}

	var req dto.UpdateBookRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	book, err := h.bookService.Update(r.Context(), id, req, getUserID(r))
	if err != nil {
		sendServiceError(w, err)
		return
	}

	utils.SendJSON(w, http.StatusOK, "Info buku berhasil diubah", book)
}

// UpdateProgress godoc
// @Summary      Simpan progres halaman baca user
// @Description  Jika halaman = total_pages, finished_at diisi otomatis.
// @Tags         Books
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        id    path      int                        true  "Book ID"
// @Param        body  body      dto.UpdateProgressRequest  true  "Halaman terakhir dibaca"
// @Success      200   {object}  map[string]interface{}
// @Failure      400   {object}  map[string]interface{}
// @Failure      403   {object}  map[string]interface{}
// @Failure      404   {object}  map[string]interface{}
// @Router       /books/{id}/progress [patch]
func (h *BookHandler) UpdateProgress(w http.ResponseWriter, r *http.Request) {
	id, ok := getIDFromURL(w, r)
	if !ok {
		return
	}

	var req dto.UpdateProgressRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	if err := h.bookService.UpdateProgress(r.Context(), id, req.LastReadPage, getUserID(r)); err != nil {
		sendServiceError(w, err)
		return
	}

	utils.SendJSON(w, http.StatusOK, "Progres baca berhasil disimpan", nil)
}

// SetFavorite godoc
// @Summary      Tandai atau hapus tanda favorit buku
// @Tags         Books
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        id    path      int                  true  "Book ID"
// @Param        body  body      dto.FavoriteRequest  true  "Status favorit"
// @Success      200   {object}  dto.BookDetailResponse
// @Failure      400   {object}  map[string]interface{}
// @Failure      403   {object}  map[string]interface{}
// @Failure      404   {object}  map[string]interface{}
// @Router       /books/{id}/favorite [put]
func (h *BookHandler) SetFavorite(w http.ResponseWriter, r *http.Request) {
	id, ok := getIDFromURL(w, r)
	if !ok {
		return
	}

	var req dto.FavoriteRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.IsFavorite == nil {
		utils.SendError(w, http.StatusBadRequest, "Kolom 'is_favorite' wajib diisi")
		return
	}

	book, err := h.bookService.SetFavorite(r.Context(), id, *req.IsFavorite, getUserID(r))
	if err != nil {
		sendServiceError(w, err)
		return
	}

	utils.SendJSON(w, http.StatusOK, "Status favorit berhasil disimpan", book)
}

// Delete godoc
// @Summary      Hapus buku
// @Description  Menghapus data buku, progres, favorit, file PDF, dan sampul. Hanya pemilik buku (selain itu 403).
// @Tags         Books
// @Security     BearerAuth
// @Produce      json
// @Param        id   path      int  true  "Book ID"
// @Success      200  {object}  map[string]interface{}
// @Failure      403  {object}  map[string]interface{}
// @Failure      404  {object}  map[string]interface{}
// @Router       /books/{id} [delete]
func (h *BookHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := getIDFromURL(w, r)
	if !ok {
		return
	}

	if err := h.bookService.Delete(r.Context(), id, getUserID(r)); err != nil {
		sendServiceError(w, err)
		return
	}

	utils.SendJSON(w, http.StatusOK, "Buku dihapus", nil)
}
