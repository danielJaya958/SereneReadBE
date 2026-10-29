package repository

import (
	"context"
	"errors"

	"booksReader/internal/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// 1. Interface: Kontrak untuk method apa saja yang ada
type UserRepository interface {
	Create(ctx context.Context, user models.User) (int, error)
	FindByEmail(ctx context.Context, email string) (models.User, error)
}

// 2. Struct: Implementasi dari interface yang memegang koneksi DB
type userRepository struct {
	db *pgxpool.Pool
}

// 3. Constructor: Fungsi untuk inisialisasi repository
func NewUserRepository(db *pgxpool.Pool) UserRepository {
	return &userRepository{db: db}
}

// 4. Implementasi Method Create (Untuk Register)
func (r *userRepository) Create(ctx context.Context, user models.User) (int, error) {
	// Query SQL dengan RETURNING id untuk mendapatkan ID yang baru dibuat
	query := `
		INSERT INTO users (username, email, password_hash)
		VALUES ($1, $2, $3)
		RETURNING id
	`

	var newID int
	// QueryRow digunakan karena kita berharap mendapat kembalian 1 baris (ID)
	err := r.db.QueryRow(ctx, query, user.Username, user.Email, user.PasswordHash).Scan(&newID)
	if err != nil {
		// Menangani error jika Email/Username sudah terdaftar (Unique Constraint)
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" { // 23505 adalah kode error Postgres untuk Unique Violation
			return 0, errors.New("email atau username sudah digunakan")
		}
		return 0, err
	}

	return newID, nil
}

// 5. Implementasi Method FindByEmail (Untuk Login)
func (r *userRepository) FindByEmail(ctx context.Context, email string) (models.User, error) {
	query := `
		SELECT id, username, email, password_hash, created_at
		FROM users
		WHERE email = $1
	`

	var user models.User
	err := r.db.QueryRow(ctx, query, email).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.PasswordHash,
		&user.CreatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return user, errors.New("user tidak ditemukan")
		}
		return user, err
	}

	return user, nil
}
