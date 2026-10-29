-- Kolom buku yang ditambahkan setelah 001 (sesuai skema database production)
ALTER TABLE books ADD COLUMN IF NOT EXISTS pdf_path VARCHAR(500) NOT NULL DEFAULT '';
ALTER TABLE books ADD COLUMN IF NOT EXISTS last_read_page INT NOT NULL DEFAULT 0;
ALTER TABLE books ADD COLUMN IF NOT EXISTS thumbnail_path VARCHAR(500) NOT NULL DEFAULT '';
ALTER TABLE books ADD COLUMN IF NOT EXISTS total_pages INT NOT NULL DEFAULT 0;
ALTER TABLE books ADD COLUMN IF NOT EXISTS is_favorite BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE books ADD COLUMN IF NOT EXISTS finished_at TIMESTAMP WITH TIME ZONE;

-- Untuk GET /books (daftar buku per user, urut terbaru)
CREATE INDEX IF NOT EXISTS idx_books_user_id_created_at ON books (user_id, created_at DESC);
