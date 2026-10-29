package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NewConnection membuat koneksi pool ke database PostgreSQL
func NewConnection(databaseUrl string) (*pgxpool.Pool, error) {
	// 1. Parsing konfigurasi dari string URL
	dbConfig, err := pgxpool.ParseConfig(databaseUrl)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database config: %w", err)
	}

	// 2. Setup konfigurasi tambahan (Opsional tapi Best Practice)
	dbConfig.MaxConns = 10                      // Maksimal koneksi simultan
	dbConfig.MinConns = 2                       // Minimal koneksi standby
	dbConfig.MaxConnLifetime = 1 * time.Hour    // Refresh koneksi setiap 1 jam
	dbConfig.MaxConnIdleTime = 30 * time.Minute // Tutup koneksi jika nganggur 30 menit

	// 3. Buka koneksi
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	dbPool, err := pgxpool.NewWithConfig(ctx, dbConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// 4. Test Ping (Pastikan DB benar-benar hidup)
	if err := dbPool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return dbPool, nil
}
