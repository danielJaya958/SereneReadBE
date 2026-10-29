package database

import (
	"context"

	"github.com/redis/go-redis/v9"
)

func NewRedisConnection(addr string) (*redis.Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr, // Dari config REDIS_ADDR (default 127.0.0.1:6379)
		Password: "",   // Kosongkan jika tidak ada password
		DB:       0,    // Database default (index 0)
	})

	// Coba ping untuk memastikan koneksi sukses
	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	return rdb, nil
}
