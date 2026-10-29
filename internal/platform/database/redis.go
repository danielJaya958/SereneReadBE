package database

import (
	"context"

	"github.com/redis/go-redis/v9"
)

func NewRedisConnection() (*redis.Client, error) {
	// Konfigurasi standar untuk localhost
	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379", // Port default Redis
		Password: "",               // Kosongkan jika tidak ada password
		DB:       0,                // Database default (index 0)
	})

	// Coba ping untuk memastikan koneksi sukses
	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	return rdb, nil
}
