package httphandler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/killerquinn/referral-system-go/internal/features/auth/dto"
)

type AuthService interface {
	RegisterUser(ctx context.Context, username string, email string, password string) (userid string, err error)
}

type HandlerRest struct {
	service AuthService
}

func NewAuthHandler(as AuthService) *HandlerRest {
	return &HandlerRest{service: as}
}

func Register(r chi.Router, as AuthService) {
	h := NewAuthHandler(as)

	r.Post("/auth/register", h.RegisterNewUser)
}

func (h *HandlerRest) RegisterNewUser(w http.ResponseWriter, r *http.Request) {
	const op = "handler.RegisterNewUser"

	if err := json.NewDecoder(r.Body).Decode(&dto.RegisterNewUserRequest); err != nil {
		http.Error(w, fmt.Sprintf("%s:%s", op, "invalid body"), http.StatusBadRequest)
		return
	}

	userID, err := h.service.RegisterUser(r.Context(), dto.RegisterNewUserRequest.Username, dto.RegisterNewUserRequest.Email, dto.RegisterNewUserRequest.Password)
	if err != nil {
		http.Error(w, fmt.Sprintf("%s:%s", op, "internal status error"), http.StatusInternalServerError)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"user_id": userID})

}
