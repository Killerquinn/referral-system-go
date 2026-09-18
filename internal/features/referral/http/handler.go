package http

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/killerquinn/referral-system-go/internal/features/referral/dto"
	httpfeatures "github.com/killerquinn/referral-system-go/internal/infrastructure/pkg/http-features"
)

type ReferralService interface {
	WhoseReferralUserIs(ctx context.Context, username string) (referrer string, referrerSince time.Time, referrerprofileURL string, err error)
}

type ReferralHandler struct {
	rService ReferralService
}

func NewRefferalHandler(rs ReferralService) *ReferralHandler {
	return &ReferralHandler{rService: rs}
}

func Register(r chi.Router, rs ReferralService) {
	rh := NewRefferalHandler(rs)

	r.Get("api/v1/referrals", rh.CurrentReferralList)
	r.Post("api/v1/referrals/draw", rh.ContestBetweenReferrals)
	r.Get("api/v1/refferer", rh.SeeWhoseReferralAlready)
}

func (rh *ReferralHandler) CurrentReferralList(w http.ResponseWriter, r *http.Request) {
	const op = "referral-system-go/internal/features/referral/http/handler.go - CurrentReferralList"

	_, ok := r.Context().Value("user_id").(string)
	if !ok {
		responseReturn(w, http.StatusUnauthorized, dto.CurrentReferralListResponse{})
		return
	}

	panic("implement me!")
}

func (rh *ReferralHandler) ContestBetweenReferrals(w http.ResponseWriter, r *http.Request) {
	const op = "referral-system-go/internal/features/referral/http/handler.go - ContestBetweenReferrals"

	_, ok := r.Context().Value("user_id").(string)
	if !ok {
		responseReturn(w, http.StatusUnauthorized, dto.ContestBetweenReferralsResponse{})
		return
	}

	panic("implement me!")
}

func (rh *ReferralHandler) SeeWhoseReferralAlready(w http.ResponseWriter, r *http.Request) {
	const op = "referral-system-go/internal/features/referral/http/handler.go - SeeWhoseReferralAlready"

	_, ok := r.Context().Value("user_id").(string)
	if !ok {
		responseReturn(w, http.StatusUnauthorized, dto.SeeWhoseReferralAlreadyResponse{Referrer: "", ReferrerURL: "", ReferralSince: time.Time{}, Message: "status: unauthorized"})
		return
	}

	var req dto.SeeWhoseReferralAlreadyRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responseReturn(w, http.StatusBadRequest, dto.SeeWhoseReferralAlreadyResponse{Referrer: "", ReferrerURL: "", ReferralSince: time.Time{}, Message: "invalid body request"})
		return
	}

	referrersUsername, referrerSince, referrerProfileUrl, err := rh.rService.WhoseReferralUserIs(r.Context(), req.Username)
	if err != nil {
		responseReturn(w, http.StatusInternalServerError, dto.SeeWhoseReferralAlreadyResponse{Referrer: "", ReferrerURL: "", ReferralSince: time.Time{}, Message: "internal server error"})
		return
	}

	response := dto.SeeWhoseReferralAlreadyResponse{
		Referrer:      referrersUsername,
		ReferralSince: referrerSince,
		ReferrerURL:   referrerProfileUrl,
	}

	responseReturn(w, http.StatusOK, response)
}

func responseReturn(w http.ResponseWriter, status int, resp any, headers ...httpfeatures.Header) {
	w.Header().Set("Content-Type", "application/json")

	for _, h := range headers {
		w.Header().Set(h.Key, h.Value)
	}

	w.WriteHeader(status)
	json.NewEncoder(w).Encode(resp)
}
