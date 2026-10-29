CREATE TABLE IF NOT EXISTS reading_sessions (
    id BIGSERIAL PRIMARY KEY,
    user_id INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    -- SET NULL: statistik tetap ada walau bukunya dihapus
    book_id INT REFERENCES books(id) ON DELETE SET NULL,
    -- UUID dibuat di HP, untuk mencegah sesi tersimpan dua kali saat dikirim ulang
    client_session_id UUID NOT NULL,
    started_at TIMESTAMP WITH TIME ZONE NOT NULL,
    ended_at TIMESTAMP WITH TIME ZONE NOT NULL,
    local_date DATE NOT NULL,
    start_page INT NOT NULL,
    end_page INT NOT NULL,
    pages_read INT NOT NULL,
    duration_seconds INT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_session_client UNIQUE (user_id, client_session_id),
    CONSTRAINT ck_session_length CHECK (duration_seconds >= 1 AND duration_seconds <= 43200),
    CONSTRAINT ck_session_pages CHECK (pages_read >= 0 AND start_page >= 1 AND end_page >= 1),
    CONSTRAINT ck_session_time CHECK (ended_at > started_at)
);

CREATE INDEX IF NOT EXISTS idx_sessions_user_date
    ON reading_sessions (user_id, local_date) INCLUDE (pages_read, duration_seconds, book_id);
