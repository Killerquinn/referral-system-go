package httpfeatures

import (
	"encoding/json"
	"net/http"
	"unicode/utf8"
)

// Username length rules shared by registration and every lookup endpoint.
const (
	UsernameMinLen = 5
	UsernameMaxLen = 20
)

// ValidUsernameLength counts runes, same as ozzo-validation's Length rule.
func ValidUsernameLength(username string) bool {
	n := utf8.RuneCountInString(username)
	return n >= UsernameMinLen && n <= UsernameMaxLen
}

// ErrorResponse is the JSON body for errors that have no feature-specific DTO
// (middleware, rate limiter, etc.), so every error the API returns is {"message": "..."}.
type ErrorResponse struct {
	Message string `json:"message"`
}

func WriteJSON(w http.ResponseWriter, status int, resp any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(resp)
}

func WriteError(w http.ResponseWriter, status int, message string) {
	WriteJSON(w, status, ErrorResponse{Message: message})
}
