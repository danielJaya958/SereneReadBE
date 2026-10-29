package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"booksReader/internal/repository"
	"booksReader/internal/utils"

	"github.com/hibiken/asynq"
)

// 1. Nama Tugas (Task Name)
const TypeGenerateThumbnail = "pdf:generate_thumbnail"

// 2. Data yang dibawa oleh tugas (Payload)
type ThumbnailPayload struct {
	BookID   int    `json:"book_id"`
	PdfPath  string `json:"pdf_path"`
	FileName string `json:"file_name"`
}

// 3. PRODUCER: Fungsi untuk mengirim tugas ke Antrean (Queue)
func NewGenerateThumbnailTask(bookID int, pdfPath string, fileName string) (*asynq.Task, error) {
	payload, err := json.Marshal(ThumbnailPayload{BookID: bookID, PdfPath: pdfPath, FileName: fileName})
	if err != nil {
		return nil, err
	}
	// Task akan antre, dan jika gagal akan diulang otomatis (MaxRetry)
	return asynq.NewTask(TypeGenerateThumbnail, payload, asynq.MaxRetry(3)), nil
}

// 4. CONSUMER (WORKER): Fungsi yang benar-benar mengerjakan tugas berat
func HandleGenerateThumbnailTask(ctx context.Context, t *asynq.Task, repo repository.BookRepository) error {
	var p ThumbnailPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("json.Unmarshal failed: %v", err)
	}

	fmt.Printf("👷 [Worker] Mulai memproses thumbnail untuk Buku ID: %d...\n", p.BookID)

	// A. Buat Folder Thumbnails jika belum ada
	thumbDir := "./uploads/thumbnails/"
	if _, err := os.Stat(thumbDir); os.IsNotExist(err) {
		os.MkdirAll(thumbDir, os.ModePerm)
	}

	// B. Panggil Poppler (Proses Berat)
	thumbFileName, err := utils.GenerateThumbnail(p.PdfPath, thumbDir, p.FileName)
	if err != nil {
		return fmt.Errorf("gagal menjalankan Poppler: %v", err)
	}

	// C. Update Database (Isi kolom thumbnail_path yang tadinya kosong)
	err = repo.UpdateThumbnailPath(ctx, p.BookID, thumbFileName)
	if err != nil {
		return fmt.Errorf("gagal update database: %v", err)
	}

	fmt.Printf("✅ [Worker] Sukses! Thumbnail %s dibuat untuk Buku ID: %d\n", thumbFileName, p.BookID)
	return nil
}
