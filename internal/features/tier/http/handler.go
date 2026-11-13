package http

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/killerquinn/referral-system-go/internal/features/tier/dto"
	httpfeatures "github.com/killerquinn/referral-system-go/internal/infrastructure/pkg/http-features"
)

type TierService interface {
	CreateTier(ctx context.Context, authorID string, req dto.CreateTierRequest) (dto.TierItem, error)
	UpdateTier(ctx context.Context, authorID, tierID string, req dto.UpdateTierRequest) error
	ArchiveTier(ctx context.Context, authorID, tierID string) error
	TiersOfUser(ctx context.Context, callerID, username string) (*dto.TierListResponse, error)
}

type TierHandler struct {
	tService TierService
}

func NewTierHandler(ts TierService) *TierHandler {
	return &TierHandler{tService: ts}
}

/*
Routes (add inside the AuthMiddleware group in rest/chi.go):

	r.Post("/api/v1/tiers", newTierHandler.CreateTier)
	r.Get("/api/v1/tiers", newTierHandler.TiersOfUser)                   // own tiers (archived included)
	r.Get("/api/v1/tiers/user/{identifier}", newTierHandler.TiersOfUser) // tiers of a user (not archived)
	r.Put("/api/v1/tiers/{tierID}", newTierHandler.UpdateTier)
	r.Delete("/api/v1/tiers/{tierID}", newTierHandler.ArchiveTier)      // archives, never deletes
*/

const (
	maxTiersPerAuthor        = 10
	maxTierNameLength        = 100
	maxTierDescriptionLength = 2000
	minPriceAmount           = 100     // 1.00 — Stripe has a minimum charge amount
	maxPriceAmount           = 100_000 // 1000.00
)

// lowercase, as Stripe expects
var allowedCurrencies = map[string]struct{}{
	"eur": {},
	"usd": {},
}

func (th *TierHandler) CreateTier(w http.ResponseWriter, r *http.Request) {
	const op = "internal/features/tier/http CreateTier"
	// TODO: userID from jwtcontext, decode dto.CreateTierRequest,
	// validate name/description length, price range, currency in allowedCurrencies, call service
	responseReturn(w, http.StatusNotImplemented, dto.CreateTierResponse{Message: "not implemented"})
}

func (th *TierHandler) UpdateTier(w http.ResponseWriter, r *http.Request) {
	const op = "internal/features/tier/http UpdateTier"
	// TODO: chi.URLParam(r, "tierID") must be a uuid; validate lengths of non-nil fields
	responseReturn(w, http.StatusNotImplemented, dto.UpdateTierResponse{Message: "not implemented"})
}

func (th *TierHandler) ArchiveTier(w http.ResponseWriter, r *http.Request) {
	const op = "internal/features/tier/http ArchiveTier"
	// TODO: chi.URLParam(r, "tierID") must be a uuid
	responseReturn(w, http.StatusNotImplemented, dto.ArchiveTierResponse{Message: "not implemented"})
}

func (th *TierHandler) TiersOfUser(w http.ResponseWriter, r *http.Request) {
	const op = "internal/features/tier/http TiersOfUser"
	// TODO: identifier optional (empty = own tiers), validate with httpfeatures.ValidUsernameLength
	responseReturn(w, http.StatusNotImplemented, dto.TierListResponse{Items: []dto.TierItem{}, Message: "not implemented"})
}

func responseReturn(w http.ResponseWriter, status int, resp any, headers ...httpfeatures.Header) {
	w.Header().Set("Content-Type", "application/json")

	for _, h := range headers {
		w.Header().Set(h.Key, h.Value)
	}

	w.WriteHeader(status)
	json.NewEncoder(w).Encode(resp)
}
