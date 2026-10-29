package middleware

import (
	"context"
	"net/http"
	"strings"

	"booksReader/internal/utils"
)

// 1. Best Practice Go: Buat tipe data khusus untuk Key Context agar tidak bentrok dengan library lain
type contextKey string

const UserIDKey contextKey = "user_id"

func AuthMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			// 2. Ambil header "Authorization"
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				utils.SendError(w, http.StatusUnauthorized, "Token tidak ditemukan")
				return
			}

			// 3. Format Token harus: "Bearer <token_string>"
			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || parts[0] != "Bearer" {
				utils.SendError(w, http.StatusUnauthorized, "Format token salah. Gunakan 'Bearer <token>'")
				return
			}

			tokenString := parts[1]

			// 4. Validasi Token menggunakan Utility yang sudah kita buat sebelumnya
			claims, err := utils.ValidateToken(tokenString)
			if err != nil {
				utils.SendError(w, http.StatusUnauthorized, "Token tidak valid atau sudah kadaluarsa")
				return
			}

			// 5. Ambil UserID dari dalam token (claims)
			// Di jwt.go kita menyimpan UserID dengan key "sub" (subject)
			userIDFloat, ok := claims["sub"].(float64) // JSON numbers di decode jadi float64 di Go
			if !ok {
				utils.SendError(w, http.StatusUnauthorized, "Gagal membaca data user dari token")
				return
			}

			userID := int(userIDFloat)

			// 6. Simpan UserID ke dalam Context request agar bisa dipakai oleh Handler (Controller) nanti
			ctx := context.WithValue(r.Context(), UserIDKey, userID)

			// 7. Lanjut ke Handler/Controller selanjutnya dengan Context yang baru
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
