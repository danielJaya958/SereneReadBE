package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"booksReader/internal/repository"
	"booksReader/internal/utils"

	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
)

// 1. Nama Tugas (Task Name)
const TypeGenerateThumbnail = "pdf:generate_thumbnail"

// 2. Data yang dibawa oleh tugas (Payload)
type ThumbnailPayload struct {
	BookID   int    `json:"book_id"`
	UserID   int    `json:"user_id"` // Untuk menghapus cache daftar buku milik user ini
	PdfPath  string `json:"pdf_path"`
	FileName string `json:"file_name"`
}

// 3. PRODUCER: Fungsi untuk mengirim tugas ke Antrean (Queue)
func NewGenerateThumbnailTask(bookID int, userID int, pdfPath string, fileName string) (*asynq.Task, error) {
	payload, err := json.Marshal(ThumbnailPayload{BookID: bookID, UserID: userID, PdfPath: pdfPath, FileName: fileName})
	if err != nil {
		return nil, err
	}
	// Task akan antre, dan jika gagal akan diulang otomatis (MaxRetry)
	return asynq.NewTask(TypeGenerateThumbnail, payload, asynq.MaxRetry(3)), nil
}

// EnqueueMissingThumbnails dijalankan sekali saat server start: buku yang sampulnya belum jadi
// (misalnya server mati sebelum worker selesai) dimasukkan lagi ke antrean.
func EnqueueMissingThumbnails(ctx context.Context, repo repository.BookRepository, client *asynq.Client) {
	books, err := repo.GetWithoutThumbnail(ctx)
	if err != nil {
		log.Printf("⚠️ Gagal mengambil buku tanpa sampul: %v", err)
		return
	}

	queued := 0
	for _, b := range books {
		pdfPath := "./uploads/pdf/" + b.PdfPath
		if _, err := os.Stat(pdfPath); err != nil {
			log.Printf("⚠️ Lewati Buku ID %d: file PDF tidak ada (%s)", b.ID, pdfPath)
			continue
		}

		task, err := NewGenerateThumbnailTask(b.ID, b.UserID, pdfPath, b.PdfPath)
		if err != nil {
			continue
		}
		if _, err := client.Enqueue(task); err == nil {
			queued++
		}
	}

	if queued > 0 {
		log.Printf("🖼️ %d buku tanpa sampul dimasukkan ke antrean worker", queued)
	}
}

// 4. CONSUMER (WORKER): Fungsi yang benar-benar mengerjakan tugas berat
func HandleGenerateThumbnailTask(ctx context.Context, t *asynq.Task, repo repository.BookRepository, rdb *redis.Client) error {
	var p ThumbnailPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("json.Unmarshal failed: %v", err)
	}

	fmt.Printf("👷 [Worker] Mulai memproses thumbnail untuk Buku ID: %d...\n", p.BookID)

	// A. Hitung jumlah halaman (tidak menggagalkan tugas jika error)
	if pages, err := utils.GetPdfPageCount(p.PdfPath); err != nil {
		fmt.Printf("⚠️ [Worker] %v\n", err)
	} else if err := repo.UpdateTotalPages(ctx, p.BookID, pages); err != nil {
		fmt.Printf("⚠️ [Worker] Gagal simpan total halaman Buku ID %d: %v\n", p.BookID, err)
	}

	// B. Buat Folder Thumbnails jika belum ada
	thumbDir := "./uploads/thumbnails/"
	if _, err := os.Stat(thumbDir); os.IsNotExist(err) {
		os.MkdirAll(thumbDir, os.ModePerm)
	}

	// C. Panggil Poppler (Proses Berat)
	thumbFileName, err := utils.GenerateThumbnail(p.PdfPath, thumbDir, p.FileName)
	if err != nil {
		return fmt.Errorf("gagal menjalankan Poppler: %v", err)
	}

	// D. Update Database (Isi kolom thumbnail_path yang tadinya kosong)
	err = repo.UpdateThumbnailPath(ctx, p.BookID, thumbFileName)
	if err != nil {
		return fmt.Errorf("gagal update database: %v", err)
	}

	// E. Hapus cache daftar buku agar thumbnail_url & total_pages terbaru langsung terlihat
	rdb.Del(ctx, fmt.Sprintf("books:user:%d", p.UserID))

	fmt.Printf("✅ [Worker] Sukses! Thumbnail %s dibuat untuk Buku ID: %d\n", thumbFileName, p.BookID)
	return nil
}
