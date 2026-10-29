package handlers

import (
	"encoding/json"
	"io" // TAMBAHAN: Untuk Copy file
	"net/http"
	"os"            // TAMBAHAN: Untuk manipulasi File/Folder
	"path/filepath" // TAMBAHAN: Untuk ekstensi file (.pdf)
	"strconv"
	"strings" // TAMBAHAN: Untuk ToLower (mengecilkan huruf)
	"time"    // TAMBAHAN: Untuk Timestamp nama file

	"booksReader/internal/dto"
	customMiddleware "booksReader/internal/middleware"
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

// Helper untuk parse ID dari URL (contoh: /books/12)
func getIDFromURL(r *http.Request) (int, error) {
	idStr := chi.URLParam(r, "id")
	return strconv.Atoi(idStr)
}

// Create godoc
// @Summary      Upload buku PDF baru
// @Tags         Books
// @Security     BearerAuth
// @Accept       multipart/form-data
// @Produce      json
// @Param        title           formData  string  true  "Judul buku"
// @Param        author          formData  string  true  "Nama pengarang"
// @Param        published_year  formData  int     true  "Tahun terbit"
// @Param        pdf_file        formData  file    true  "File PDF (Max 75MB)"
// @Success      201  {object}  map[string]interface{}
// @Failure      400  {object}  map[string]interface{}
// @Failure      500  {object}  map[string]interface{}
// @Router       /books [post]
func (h *BookHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(customMiddleware.UserIDKey).(int)

	// 1. Batasi ukuran file maksimal 75 MB
	err := r.ParseMultipartForm(75 << 20)
	if err != nil {
		utils.SendError(w, http.StatusBadRequest, "Ukuran file terlalu besar (Max 75MB)") // Perbaikan pesan error
		return
	}

	// 2. Ambil data teks dari Form
	title := r.FormValue("title")
	author := r.FormValue("author")
	yearStr := r.FormValue("published_year")
	year, _ := strconv.Atoi(yearStr)

	// Validasi data teks
	if title == "" || author == "" || year == 0 {
		utils.SendError(w, http.StatusBadRequest, "Title, Author, dan Published Year wajib diisi")
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
	defer dest.Close()

	// Copy isi file yang diupload ke file baru di server
	if _, err := io.Copy(dest, file); err != nil {
		utils.SendError(w, http.StatusInternalServerError, "Gagal menulis file")
		return
	}

	// 7. Simpan Info Buku ke DB & Kirim Tugas ke Worker
	// Parameter 'thumbFileName' sudah tidak diperlukan lagi di sini.
	err = h.bookService.Create(r.Context(), title, author, year, fileName, userID)
	if err != nil {
		// Rollback: Hapus PDF jika gagal simpan ke DB atau gagal kirim ke Redis
		os.Remove(fullPath)
		utils.SendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// 8. Kirim Respons Cepat ke User
	utils.SendJSON(w, http.StatusCreated, "Buku PDF berhasil diupload! Sampul sedang diproses di background.", nil)
}

// GetAll godoc
// @Summary      Ambil semua daftar buku
// @Tags         Books
// @Security     BearerAuth
// @Produce      json
// @Success      200  {object}  map[string]interface{}
// @Failure      500  {object}  map[string]interface{}
// @Router       /books [get]
func (h *BookHandler) GetAll(w http.ResponseWriter, r *http.Request) {
	books, err := h.bookService.GetAll(r.Context())
	if err != nil {
		utils.SendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	utils.SendJSON(w, http.StatusOK, "Berhasil mengambil daftar buku", books)
}

// GetDetail godoc
// @Summary      Ambil detail satu buku
// @Tags         Books
// @Security     BearerAuth
// @Produce      json
// @Param        id   path      int  true  "Book ID"
// @Success      200  {object}  map[string]interface{}
// @Failure      404  {object}  map[string]interface{}
// @Router       /books/{id} [get]
func (h *BookHandler) GetDetail(w http.ResponseWriter, r *http.Request) {
	id, err := getIDFromURL(r)
	if err != nil {
		utils.SendError(w, http.StatusBadRequest, "ID tidak valid")
		return
	}

	book, err := h.bookService.GetDetail(r.Context(), id)
	if err != nil {
		utils.SendError(w, http.StatusNotFound, err.Error())
		return
	}

	utils.SendJSON(w, http.StatusOK, "Berhasil mengambil detail buku", book)
}

// GetContent godoc
// @Summary      Ambil URL PDF buku
// @Tags         Books
// @Security     BearerAuth
// @Produce      json
// @Param        id   path      int  true  "Book ID"
// @Success      200  {object}  map[string]string
// @Failure      404  {object}  map[string]interface{}
// @Router       /books/{id}/content [get]
func (h *BookHandler) GetContent(w http.ResponseWriter, r *http.Request) {
	id, err := getIDFromURL(r)
	if err != nil {
		utils.SendError(w, http.StatusBadRequest, "ID tidak valid")
		return
	}

	pdfUrl, err := h.bookService.GetContent(r.Context(), id)
	if err != nil {
		utils.SendError(w, http.StatusNotFound, err.Error())
		return
	}

	// PERBAIKAN MINOR: Ubah key json menjadi "pdf_url" agar lebih jelas bagi Frontend
	utils.SendJSON(w, http.StatusOK, "Berhasil membaca isi buku", map[string]string{"pdf_url": pdfUrl})
}

// Update godoc
// @Summary      Update metadata buku (judul, author, tahun)
// @Tags         Books
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        id    path      int              true  "Book ID"
// @Param        body  body      dto.BookRequest  true  "Data buku yang diupdate"
// @Success      200   {object}  map[string]interface{}
// @Failure      403   {object}  map[string]interface{}
// @Failure      500   {object}  map[string]interface{}
// @Router       /books/{id} [put]
// Update ini hanya untuk mengupdate metadata teks (Judul, Author, Tahun), bukan ganti file PDF-nya.
func (h *BookHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(customMiddleware.UserIDKey).(int)
	id, _ := getIDFromURL(r)

	var req dto.BookRequest
	json.NewDecoder(r.Body).Decode(&req)

	err := h.bookService.Update(r.Context(), id, req, userID)
	if err != nil {
		if err.Error() == "anda tidak memiliki izin untuk mengubah buku ini" {
			utils.SendError(w, http.StatusForbidden, err.Error()) // 403 Forbidden
			return
		}
		utils.SendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SendJSON(w, http.StatusOK, "Metadata buku berhasil diupdate", nil)
}

// UpdateProgress godoc
// @Summary      Simpan progres halaman baca user
// @Tags         Books
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        id    path      int                       true  "Book ID"
// @Param        body  body      dto.UpdateProgressRequest true  "Halaman terakhir dibaca"
// @Success      200   {object}  map[string]interface{}
// @Failure      500   {object}  map[string]interface{}
// @Router       /books/{id}/progress [put]
func (h *BookHandler) UpdateProgress(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(customMiddleware.UserIDKey).(int)
	id, _ := getIDFromURL(r)

	var req dto.UpdateProgressRequest
	json.NewDecoder(r.Body).Decode(&req)

	err := h.bookService.UpdateProgress(r.Context(), id, req.LastReadPage, userID)
	if err != nil {
		utils.SendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SendJSON(w, http.StatusOK, "Progres baca berhasil disimpan", nil)
}

// Delete godoc
// @Summary      Hapus buku
// @Tags         Books
// @Security     BearerAuth
// @Produce      json
// @Param        id   path      int  true  "Book ID"
// @Success      200  {object}  map[string]interface{}
// @Failure      403  {object}  map[string]interface{}
// @Failure      500  {object}  map[string]interface{}
// @Router       /books/{id} [delete]
func (h *BookHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(customMiddleware.UserIDKey).(int)
	id, _ := getIDFromURL(r)

	err := h.bookService.Delete(r.Context(), id, userID)
	if err != nil {
		if err.Error() == "anda tidak memiliki izin untuk menghapus buku ini" {
			utils.SendError(w, http.StatusForbidden, err.Error())
			return
		}
		utils.SendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SendJSON(w, http.StatusOK, "Buku berhasil dihapus", nil)
}
