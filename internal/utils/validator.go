package utils

import (
	"fmt"

	"github.com/go-playground/validator/v10"
)

// Inisialisasi satu instance validator untuk dipakai seluruh aplikasi
var Validate *validator.Validate

func init() {
	Validate = validator.New()
}

// FormatValidationError mengubah error mentah dari library menjadi pesan yang enak dibaca user
func FormatValidationError(err error) string {
	var errorMessage string
	for _, err := range err.(validator.ValidationErrors) {
		switch err.Tag() {
		case "required":
			errorMessage = fmt.Sprintf("Kolom '%s' wajib diisi.", err.Field())
		case "email":
			errorMessage = fmt.Sprintf("Kolom '%s' harus berupa email yang valid.", err.Field())
		case "min":
			errorMessage = fmt.Sprintf("Kolom '%s' minimal %s karakter.", err.Field(), err.Param())
		case "max":
			errorMessage = fmt.Sprintf("Kolom '%s' maksimal %s karakter.", err.Field(), err.Param())
		default:
			errorMessage = fmt.Sprintf("Kolom '%s' tidak valid.", err.Field())
		}
		break // Kita ambil error pertama saja agar user tidak pusing
	}
	return errorMessage
}
