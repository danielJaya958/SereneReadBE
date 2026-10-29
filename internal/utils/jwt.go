package utils

import (
	"errors"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Mengambil secret key dari .env
func getSecretKey() []byte {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "fallback_secret_key" // Fallback darurat jika .env tidak terbaca
	}
	return []byte(secret)
}

// GenerateToken membuat JWT token yang berisi User ID
func GenerateToken(userID int) (string, error) {
	// 1. Tentukan isi token (Claims)
	// Kita menyimpan "sub" (subject/UserID) dan "exp" (waktu kedaluwarsa)
	claims := jwt.MapClaims{
		"sub": userID,
		"exp": time.Now().Add(24 * time.Hour).Unix(), // Token berlaku 24 jam
		"iat": time.Now().Unix(),                     // Waktu token dibuat
	}

	// 2. Buat token dengan algoritma HS256
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// 3. Tanda tangani token dengan Secret Key
	return token.SignedString(getSecretKey())
}

// ValidateToken akan digunakan nanti di Middleware untuk mengecek keaslian token
func ValidateToken(tokenString string) (jwt.MapClaims, error) {
	// Parse dan validasi token
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Pastikan algoritma yang digunakan adalah HMAC (HS256)
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return getSecretKey(), nil
	})

	if err != nil {
		return nil, err
	}

	// Ambil isinya (claims) jika token valid
	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid token")
}
