package handlers

import (
	"encoding/json"
	"net/http"

	"booksReader/internal/dto"
	"booksReader/internal/service"
	"booksReader/internal/utils"
)

// 1. Struct Handler yang memegang dependensi Service
type UserHandler struct {
	userService service.UserService
}

// 2. Constructor
func NewUserHandler(srv service.UserService) *UserHandler {
	return &UserHandler{userService: srv}
}

// 3. Method Register
// Register godoc
// @Summary      Registrasi user baru
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        body  body      dto.RegisterRequest  true  "Data registrasi"
// @Success      201   {object}  map[string]interface{}
// @Failure      400   {object}  map[string]interface{}
// @Failure      409   {object}  map[string]interface{}
// @Router       /register [post]
func (h *UserHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req dto.RegisterRequest

	// 1. Decode JSON
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.SendError(w, http.StatusBadRequest, "Format JSON tidak valid")
		return
	}

	// 2. VALIDASI OTOMATIS: Cek aturan min, max, required, email
	if err := utils.Validate.Struct(req); err != nil {
		utils.SendError(w, http.StatusBadRequest, utils.FormatValidationError(err))
		return
	}

	// Panggil Service Layer
	err := h.userService.Register(r.Context(), req)
	if err != nil {
		// Jika error mengandung kata "sudah digunakan" (dari repository), kirim Conflict (409)
		if err.Error() == "email atau username sudah digunakan" {
			utils.SendError(w, http.StatusConflict, err.Error())
			return
		}
		utils.SendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Kirim Respons Sukses
	utils.SendJSON(w, http.StatusCreated, "Registrasi berhasil", nil)
}

// 4. Method Login
// Login godoc
// @Summary      Login user
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        body  body      dto.LoginRequest  true  "Kredensial login"
// @Success      200   {object}  map[string]interface{}
// @Failure      400   {object}  map[string]interface{}
// @Failure      401   {object}  map[string]interface{}
// @Router       /login [post]
func (h *UserHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req dto.LoginRequest

	// 1. Decode JSON
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.SendError(w, http.StatusBadRequest, "Format JSON tidak valid")
		return
	}

	// 2. VALIDASI OTOMATIS
	if err := utils.Validate.Struct(req); err != nil {
		utils.SendError(w, http.StatusBadRequest, utils.FormatValidationError(err))
		return
	}

	// Panggil Service Layer untuk mendapatkan Token
	loginResp, err := h.userService.Login(r.Context(), req)
	if err != nil {
		utils.SendError(w, http.StatusUnauthorized, err.Error()) // 401 Unauthorized
		return
	}

	// Kirim Respons Sukses beserta Token JWT
	utils.SendJSON(w, http.StatusOK, "Login berhasil", loginResp)
}
