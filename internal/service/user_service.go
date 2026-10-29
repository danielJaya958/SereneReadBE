package service

import (
	"context"
	"errors"

	"booksReader/internal/dto"
	"booksReader/internal/models"
	"booksReader/internal/repository"
	"booksReader/internal/utils"

	"golang.org/x/crypto/bcrypt"
)

// 1. Interface Service
type UserService interface {
	Register(ctx context.Context, req dto.RegisterRequest) error
	Login(ctx context.Context, req dto.LoginRequest) (dto.LoginResponse, error)
}

// 2. Struct Implementasi
type userService struct {
	userRepo repository.UserRepository
}

// 3. Constructor
func NewUserService(repo repository.UserRepository) UserService {
	return &userService{userRepo: repo}
}

// 4. Implementasi Logika Register
func (s *userService) Register(ctx context.Context, req dto.RegisterRequest) error {
	// a. Validasi Sederhana (Bisa diganti dengan library validator nanti)
	if req.Username == "" || req.Email == "" || len(req.Password) < 6 {
		return errors.New("data tidak lengkap atau password kurang dari 6 karakter")
	}

	// b. Hashing Password menggunakan Bcrypt
	// Cost 10 adalah standar yang baik (seimbang antara keamanan dan kecepatan)
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), 10)
	if err != nil {
		return errors.New("gagal memproses password")
	}

	// c. Mapping dari DTO ke Model
	newUser := models.User{
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: string(hashedPassword),
	}

	// d. Simpan ke Database via Repository
	_, err = s.userRepo.Create(ctx, newUser)
	if err != nil {
		return err // Error seperti "email sudah terdaftar" dari repo akan dilempar ke sini
	}

	return nil
}

// 5. Implementasi Logika Login
func (s *userService) Login(ctx context.Context, req dto.LoginRequest) (dto.LoginResponse, error) {
	// a. Cari user berdasarkan email
	user, err := s.userRepo.FindByEmail(ctx, req.Email)
	if err != nil {
		return dto.LoginResponse{}, errors.New("email atau password salah") // Jangan beritahu "Email tidak ditemukan" demi keamanan
	}

	// b. Bandingkan Password yang diketik user dengan Hash di DB
	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password))
	if err != nil {
		return dto.LoginResponse{}, errors.New("email atau password salah")
	}

	// c. Generate JWT Token yang sesungguhnya (UBAH BAGIAN INI)
	token, err := utils.GenerateToken(user.ID)
	if err != nil {
		return dto.LoginResponse{}, errors.New("gagal membuat token login")
	}

	return dto.LoginResponse{Token: token}, nil
}
