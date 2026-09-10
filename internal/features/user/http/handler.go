package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/killerquinn/referral-system-go/internal/features/user/dto"
	sharederrors "github.com/killerquinn/referral-system-go/internal/shared/shared-errors.go"
)

type UserService interface {
	CompareAndChangePassword(ctx context.Context, userID string, oldPassword string, newPassword string) (err error)
}
type UserHandler struct {
	uService UserService
}

func NewUserHandler(us UserService) *UserHandler {
	return &UserHandler{uService: us}
}

func Register(r chi.Router, us UserService) {
	h := NewUserHandler(us)

	r.Put("/user/changepassword", h.ChangeUserPassword)
}

var (
	ErrInvalidCreds = errors.New("Error invalid credentials")
)

func (uh *UserHandler) ChangeUserPassword(w http.ResponseWriter, r *http.Request) {
	const op = "user/http.ChangeUserPassword"

	userID, ok := r.Context().Value("user_id").(string)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req dto.ChangeUserPasswordRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	if err := uh.uService.CompareAndChangePassword(r.Context(), userID, req.OldPass, req.NewPass); err != nil {
		if errors.Is(err, sharederrors.ErrInvalidCreds) {
			http.Error(w, "Invalid credentials", http.StatusForbidden)
			return
		}
		http.Error(w, "status internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(http.StatusOK)

}
