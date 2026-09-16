package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/killerquinn/referral-system-go/internal/features/user/dto"
	sharederrors "github.com/killerquinn/referral-system-go/internal/shared/shared-errors.go"
)

type UserService interface {
	CompareAndChangePassword(ctx context.Context, userID string, oldPassword string, newPassword string) (err error)
	ChangeUsersCurrentReferrer(ctx context.Context, userID string, referralString string) (newTryWillBeAfter time.Time, err error)
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
	r.Put("/user/changerefferer", h.ChangeReferrer)
}

var (
	ErrInvalidCreds = errors.New("Error invalid credentials")
)

func (uh *UserHandler) ChangeUserPassword(w http.ResponseWriter, r *http.Request) {
	const op = "/internal/features/user/http.ChangeUserPassword"

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

	if err := ValidateChangePassword(&req); err != nil {
		http.Error(w, fmt.Sprintf("invalid body format: %v", err), http.StatusBadRequest)
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

func (uh *UserHandler) ChangeReferrer(w http.ResponseWriter, r *http.Request) {
	const op = "/internal/features/user/http.ChangeReferrer"

	userID, ok := r.Context().Value("user_id").(string)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	}

	var req dto.ChangeReferrerRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("invalid body format: %v", err), http.StatusBadRequest)
		return
	}

	refCooldown, err := uh.uService.ChangeUsersCurrentReferrer(r.Context(), userID, req.ReferralCode)
	if err != nil {
		if errors.Is(err, sharederrors.ErrReferrerOrReferralCodeDoesntExist) {
			http.Error(w, fmt.Sprintf("failed while trying change referrer: %v", sharederrors.ErrReferrerOrReferralCodeDoesntExist), http.StatusBadRequest)
			return
		}
		http.Error(w, "status internal server error", http.StatusInternalServerError)
		return
	}

	resp := dto.ChangeReferrerResponse{
		ReferralCooldown: refCooldown,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)

}
