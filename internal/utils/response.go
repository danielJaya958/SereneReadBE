package utils

import (
	"encoding/json"
	"net/http"
)

// StandardResponse adalah format baku untuk semua balasan API kita
type StandardResponse struct {
	Status  bool        `json:"status"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// SendJSON adalah helper untuk mengirim sukses respons
func SendJSON(w http.ResponseWriter, statusCode int, message string, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(StandardResponse{
		Status:  true,
		Message: message,
		Data:    data,
	})
}

// SendError adalah helper untuk mengirim error respons
func SendError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(StandardResponse{
		Status:  false,
		Message: message,
		Data:    nil,
	})
}
