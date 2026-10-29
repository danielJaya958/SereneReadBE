package middleware

import (
	"context"
	"net/http"
	"sync"
)

// 1. Kita buat struct khusus untuk menyimpan fungsi cancel
type RequestEntry struct {
	cancel context.CancelFunc
}

// 2. Ubah tipe map untuk menyimpan Pointer dari RequestEntry
var activeRequests = make(map[string]*RequestEntry)
var mu sync.Mutex

func CancelPrevious() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			clientID := r.RemoteAddr

			mu.Lock()
			// Cek apakah ada request lama. Jika ada, batalkan.
			if oldEntry, exists := activeRequests[clientID]; exists {
				oldEntry.cancel()
			}

			// Buat Context baru
			ctx, newCancel := context.WithCancel(r.Context())

			// 3. Bungkus fungsi cancel ke dalam Pointer
			currentEntry := &RequestEntry{cancel: newCancel}

			// Simpan Pointer tersebut ke map
			activeRequests[clientID] = currentEntry
			mu.Unlock()

			// Bersihkan memori setelah selesai
			defer func() {
				mu.Lock()
				// 4. Sekarang kita BISA membandingkan Pointer (currentEntry)
				if activeRequests[clientID] == currentEntry {
					delete(activeRequests, clientID)
				}
				mu.Unlock()
				newCancel()
			}()

			// Lanjut ke Handler
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
