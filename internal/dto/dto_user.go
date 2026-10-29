package dto

// Request untuk Register
type RegisterRequest struct {
	Username string `json:"username" validate:"required,min=3,max=50"`
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=6"`
}

// Request untuk Login
type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

// Response setelah berhasil Login (Nanti akan berisi Token JWT)
type LoginResponse struct {
	Token string `json:"token"`
}
