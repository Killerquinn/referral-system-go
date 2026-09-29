package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/killerquinn/referral-system-go/internal/features/referral/dto"
	httpfeatures "github.com/killerquinn/referral-system-go/internal/infrastructure/pkg/http-features"
	sharederrors "github.com/killerquinn/referral-system-go/internal/shared/shared-errors.go"
)

type ReferralService interface {
	WhoseReferralUserIs(ctx context.Context, username string) (referrer string, referrerprofileURL string, referrerSince time.Time, err error)
	CheckCurrentReferralList(ctx context.Context, referrerID string, params dto.ReferralListParams) (*dto.CurrentReferralListResponse, error)
	StartContest(ctx context.Context, sponsorID string, winnersQuantity int, startFrom *time.Time, endUntil *time.Time) (dto.ContestBetweenReferralsResponse, error)
}

type ReferralHandler struct {
	rService ReferralService
}

func NewRefferalHandler(rs ReferralService) *ReferralHandler {
	return &ReferralHandler{rService: rs}
}

func Register(r chi.Router, rs ReferralService) {
	rh := NewRefferalHandler(rs)

	r.Get("api/v1/referrals/{identifier}", rh.CurrentReferralList)
	r.Post("api/v1/referrals/draw", rh.ContestBetweenReferrals)
	r.Get("api/v1/{identifier}/refferer", rh.SeeWhoseReferralAlready)
}

const (
	defaultLimit       = 20
	maxLimit           = 100
	minWinnersQuantity = 1
	maxWinnersQuantity = 100
)

func (rh *ReferralHandler) CurrentReferralList(w http.ResponseWriter, r *http.Request) { // cursor based pagination
	const op = "referral-system-go/internal/features/referral/http/handler.go - CurrentReferralList"

	var targetID string

	userID, ok := r.Context().Value("user_id").(string) //use it in case if identifier is empty
	if !ok {
		responseReturn(w, http.StatusUnauthorized, dto.CurrentReferralListResponse{Items: nil, NextCursor: nil, HasMore: false, Message: "cannot check referrals: unauthorized"})
		return
	}

	identifier := chi.URLParam(r, "identifier")
	cursor := r.URL.Query().Get("cursor")
	limitStr := r.URL.Query().Get("limit")

	parsedLimit := parseLimit(limitStr)

	if identifier == "" {
		targetID = userID
	} else {
		_, err := uuid.Parse(identifier)
		if err == nil {
			targetID = userID //not allowing user to find someone by userID
		} else {
			if len(identifier) < 3 || len(identifier) > 32 {

				http.Error(w, "invalid username length", http.StatusBadRequest)
				return
			}
			targetID = identifier
		}
	}

	req := dto.CurrentReferralListRequest{
		Identifier: targetID,
		Cursor:     cursor,
		Limit:      parsedLimit,
	}
	params := dto.ReferralListParams{
		Limit:  req.Limit,
		Cursor: req.Cursor,
	}

	resp, err := rh.rService.CheckCurrentReferralList(r.Context(), req.Identifier, params)
	if err != nil {
		if errors.Is(err, sharederrors.ErrUserNotFound) {
			responseReturn(w, http.StatusNotFound, dto.CurrentReferralListResponse{Items: nil, NextCursor: nil, HasMore: false, Message: "user with this username doesnt exist"})
			return
		}
		if errors.Is(err, sharederrors.ErrUserIsBanned) {
			responseReturn(w, http.StatusForbidden, dto.CurrentReferralListResponse{Items: nil, NextCursor: nil, HasMore: false, Message: "Error user is banned. Any interactions with this user unavaible now"})
			return
		}
		responseReturn(w, http.StatusInternalServerError, dto.CurrentReferralListResponse{Items: nil, NextCursor: nil, HasMore: false, Message: "internal server error"})
		return
	}

	responseReturn(w, http.StatusOK, resp)
}

func (rh *ReferralHandler) ContestBetweenReferrals(w http.ResponseWriter, r *http.Request) {
	const op = "referral-system-go/internal/features/referral/http/handler.go - ContestBetweenReferrals"

	sponsorID, ok := r.Context().Value("user_id").(string)
	if !ok {
		responseReturn(w, http.StatusUnauthorized, dto.ContestBetweenReferralsResponse{Winners: nil, Message: "unauthorized"})
		return
	}

	var req dto.ContestBetweenReferralsRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responseReturn(w, http.StatusBadRequest, dto.ContestBetweenReferralsResponse{Winners: nil, Message: "invalid request body"})
		return
	}

	if req.WinnersQuantity < minWinnersQuantity ||
		req.WinnersQuantity > maxWinnersQuantity {
		responseReturn(w, http.StatusBadRequest, dto.ContestBetweenReferralsResponse{Winners: nil, Message: "invalid winners quantity"})
		return
	}

	if req.StartFrom != nil && req.EndUntil != nil &&
		req.StartFrom.After(*req.EndUntil) {
		responseReturn(w, http.StatusBadRequest, dto.ContestBetweenReferralsResponse{Winners: nil, Message: "winners_became_referrals_since must be before referrals_can_be_winners_until"})
		return
	}

	resp, err := rh.rService.StartContest(r.Context(), sponsorID, req.WinnersQuantity, req.StartFrom, req.EndUntil)
	if err != nil {

		responseReturn(w, http.StatusInternalServerError, dto.ContestBetweenReferralsResponse{Winners: nil, Message: "status internal server error"})
		return
	}

	responseReturn(w, http.StatusOK, resp)
}

func (rh *ReferralHandler) SeeWhoseReferralAlready(w http.ResponseWriter, r *http.Request) {
	const op = "referral-system-go/internal/features/referral/http/handler.go - SeeWhoseReferralAlready"

	_, ok := r.Context().Value("user_id").(string)
	if !ok {
		responseReturn(w, http.StatusUnauthorized, dto.SeeWhoseReferralAlreadyResponse{Referrer: "", ReferrerURL: "", ReferralSince: time.Time{}, Message: "status: unauthorized"})
		return
	}

	identifier := chi.URLParam(r, "identifier")

	req := dto.SeeWhoseReferralAlreadyRequest{
		Username: identifier,
	}

	referrersUsername, referrerProfileUrl, referrerSince, err := rh.rService.WhoseReferralUserIs(r.Context(), req.Username)
	if err != nil {
		if errors.Is(err, sharederrors.ErrUserNotFound) {
			responseReturn(w, http.StatusNotFound, dto.SeeWhoseReferralAlreadyResponse{Referrer: "", ReferrerURL: "", ReferralSince: time.Time{}, Message: "User not found"})
			return

		}
		if errors.Is(err, sharederrors.ErrUserIsNotReferredYet) {
			responseReturn(w, http.StatusNoContent, dto.SeeWhoseReferralAlreadyResponse{Referrer: "", ReferrerURL: "", ReferralSince: time.Time{}, Message: "user is not referred yet to find his referrer"})
			return

		}
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

func parseLimit(limitStr string) int {
	if limitStr == "" {
		return defaultLimit
	}

	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		return defaultLimit
	}

	if limit <= 0 {
		return defaultLimit
	}

	if limit > maxLimit {
		return maxLimit
	}

	return limit
}
