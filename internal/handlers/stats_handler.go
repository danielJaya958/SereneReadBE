package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"booksReader/internal/dto"
	"booksReader/internal/service"
	"booksReader/internal/utils"
)

const maxSessionsPerRequest = 100

type StatsHandler struct {
	statsService service.StatsService
}

func NewStatsHandler(srv service.StatsService) *StatsHandler {
	return &StatsHandler{statsService: srv}
}

// SaveSessions godoc
// @Summary      Kirim sesi membaca (batch)
// @Description  Maksimal 100 sesi. Setiap sesi divalidasi terpisah: sesi valid disimpan walau sesi lain ditolak. client_session_id yang sudah tersimpan dihitung sebagai duplicates. 400 hanya untuk body yang tidak bisa dibaca.
// @Tags         Statistics
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        body  body      dto.ReadingSessionsRequest  true  "Daftar sesi membaca"
// @Success      200   {object}  dto.ReadingSessionsResult
// @Failure      400   {object}  map[string]interface{}
// @Router       /reading-sessions [post]
func (h *StatsHandler) SaveSessions(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // Maksimal 1 MB

	var req dto.ReadingSessionsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.SendError(w, http.StatusBadRequest, `Format JSON tidak valid. Kirim {"sessions": [...]}`)
		return
	}
	if req.Sessions == nil {
		utils.SendError(w, http.StatusBadRequest, "Kolom 'sessions' wajib berupa array")
		return
	}
	if len(req.Sessions) > maxSessionsPerRequest {
		utils.SendError(w, http.StatusBadRequest, fmt.Sprintf("Maksimal %d sesi per request", maxSessionsPerRequest))
		return
	}

	result, err := h.statsService.SaveSessions(r.Context(), req.Sessions, getUserID(r))
	if err != nil {
		sendServiceError(w, err)
		return
	}

	utils.SendJSON(w, http.StatusOK, "Sesi membaca disimpan", result)
}

// Summary godoc
// @Summary      Ringkasan statistik membaca
// @Description  today = tanggal lokal HP, dipakai untuk "hari ini", "minggu ini" (Senin-Minggu), dan streak. User tanpa sesi mendapat angka 0 dan last_read_date null.
// @Tags         Statistics
// @Security     BearerAuth
// @Produce      json
// @Param        today  query     string  true  "Tanggal lokal HP (YYYY-MM-DD)"
// @Success      200    {object}  dto.StatsSummaryResponse
// @Failure      400    {object}  map[string]interface{}
// @Router       /stats/summary [get]
func (h *StatsHandler) Summary(w http.ResponseWriter, r *http.Request) {
	summary, err := h.statsService.Summary(r.Context(), r.URL.Query().Get("today"), getUserID(r))
	if err != nil {
		sendServiceError(w, err)
		return
	}
	utils.SendJSON(w, http.StatusOK, "Berhasil mengambil statistik", summary)
}

// Daily godoc
// @Summary      Statistik membaca per hari
// @Description  Satu item untuk setiap tanggal dari from sampai to, termasuk hari tanpa membaca (diisi 0). Rentang maksimal 366 hari.
// @Tags         Statistics
// @Security     BearerAuth
// @Produce      json
// @Param        from  query     string  true  "Tanggal awal (YYYY-MM-DD)"
// @Param        to    query     string  true  "Tanggal akhir (YYYY-MM-DD)"
// @Success      200   {array}   dto.DailyStat
// @Failure      400   {object}  map[string]interface{}
// @Router       /stats/daily [get]
func (h *StatsHandler) Daily(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	days, err := h.statsService.Daily(r.Context(), query.Get("from"), query.Get("to"), getUserID(r))
	if err != nil {
		sendServiceError(w, err)
		return
	}
	utils.SendJSON(w, http.StatusOK, "Berhasil mengambil statistik harian", days)
}

// Books godoc
// @Summary      Buku yang paling lama dibaca
// @Description  Urut dari menit terbanyak. from/to opsional (tanpa keduanya = semua waktu). Buku yang sudah dihapus tidak ikut.
// @Tags         Statistics
// @Security     BearerAuth
// @Produce      json
// @Param        from   query     string  false  "Tanggal awal (YYYY-MM-DD)"
// @Param        to     query     string  false  "Tanggal akhir (YYYY-MM-DD)"
// @Param        limit  query     int     false  "1-50, bawaan 5"
// @Success      200    {array}   dto.BookStat
// @Failure      400    {object}  map[string]interface{}
// @Router       /stats/books [get]
func (h *StatsHandler) Books(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	limit, ok := parseQueryInt(w, query, "limit")
	if !ok {
		return
	}

	books, err := h.statsService.Books(r.Context(), query.Get("from"), query.Get("to"), limit, getUserID(r))
	if err != nil {
		sendServiceError(w, err)
		return
	}
	utils.SendJSON(w, http.StatusOK, "Berhasil mengambil statistik buku", books)
}
