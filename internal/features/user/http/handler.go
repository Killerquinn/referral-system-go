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
	httpfeatures "github.com/killerquinn/referral-system-go/internal/infrastructure/pkg/http-features"
	sharederrors "github.com/killerquinn/referral-system-go/internal/shared/shared-errors.go"
)

type UserService interface {
	CompareAndChangePassword(ctx context.Context, userID string, oldPassword string, newPassword string) (err error)
	ChangeUsersCurrentReferrer(ctx context.Context, userID string, referralString string) (newTryWillBeAfter time.Duration, err error)
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
		responseReturn(w, http.StatusUnauthorized, dto.ChangeReferrerResponse{ReferralCooldown: "", Message: "status: unauthorized"})

		return
	}

	var req dto.ChangeReferrerRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responseReturn(w, http.StatusBadRequest, dto.ChangeReferrerResponse{ReferralCooldown: "", Message: "invalid request body"})

		return
	}

	refCooldown, err := uh.uService.ChangeUsersCurrentReferrer(r.Context(), userID, req.ReferralCode)
	if err != nil {
		if errors.Is(err, sharederrors.ErrCooldownNotPassedYet) {
			totalHours := int(refCooldown / time.Hour)
			days := totalHours / 24
			hours := totalHours % 24

			resp := dto.ChangeReferrerResponse{
				ReferralCooldown: fmt.Sprintf("%d days, %d hours", days, hours),
				Message:          "Referrer was't changed due cooldown",
			}

			responseReturn(w, http.StatusTooManyRequests, resp)

			return
		}

		if errors.Is(err, sharederrors.ErrReferrerOrReferralCodeDoesntExist) {
			resp := dto.ChangeReferrerResponse{
				ReferralCooldown: "cooldown hasnt been checked",
				Message:          "Referrer wasnt changed, referrer or referral code doesnt exist",
			}

			responseReturn(w, http.StatusBadRequest, resp)

			return
		}

		http.Error(w, "status internal server error", http.StatusInternalServerError)

		return
	}

	totalHours := int(refCooldown / time.Hour)
	days := totalHours / 24
	hours := totalHours % 24

	resp := dto.ChangeReferrerResponse{
		ReferralCooldown: fmt.Sprintf("%d days, %d hours", days, hours),
		Message:          "Referrer successfully changed. New cooldown is set to 30 days",
	}

	responseReturn(w, http.StatusOK, resp)

}

func responseReturn(w http.ResponseWriter, status int, resp any, headers ...httpfeatures.Header) {
	w.Header().Set("Content-Type", "application/json")

	for _, h := range headers {
		w.Header().Set(h.Key, h.Value)
	}

	w.WriteHeader(status)
	json.NewEncoder(w).Encode(resp)
}
