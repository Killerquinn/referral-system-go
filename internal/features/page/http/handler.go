package http

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/killerquinn/referral-system-go/internal/features/page/dto"
	httpfeatures "github.com/killerquinn/referral-system-go/internal/infrastructure/pkg/http-features"
)

type PageService interface {
	// viewerID == "" for an anonymous visitor
	GetPublicPage(ctx context.Context, viewerID, slug string) (*dto.PublicPage, error)
	GetMyPage(ctx context.Context, userID string) (dto.PageItem, error)
	CreatePage(ctx context.Context, userID string, req dto.CreatePageRequest) (dto.PageItem, error)
	UpdateMyPage(ctx context.Context, userID string, req dto.UpdatePageRequest) (dto.PageItem, error)
	ChangeSlug(ctx context.Context, userID string, req dto.ChangeSlugRequest) error
	CheckSlug(ctx context.Context, slug string) (dto.SlugAvailabilityResponse, error)
}

type PageHandler struct {
	pService PageService
}

func NewPageHandler(ps PageService) *PageHandler {
	return &PageHandler{pService: ps}
}

/*
Routes (rest/chi.go):

	// public group — OptionalAuthMiddleware: token is optional, no 401
	r.Group(func(r chi.Router) {
		r.Use(localmdw.OptionalAuthMiddleware(cfg.JWT.Secret))
		r.Get("/api/v1/pages/{slug}", newPageHandler.GetPublicPage)
	})

	// inside the AuthMiddleware group
	// (static segments "me" / "slug-available" win over {slug} in chi; both are also reserved slugs)
	r.Get("/api/v1/pages/me", newPageHandler.GetMyPage)
	r.Post("/api/v1/pages", newPageHandler.CreatePage)
	r.Put("/api/v1/pages/me", newPageHandler.UpdateMyPage)
	r.Put("/api/v1/pages/me/slug", newPageHandler.ChangeSlug)
	r.Get("/api/v1/pages/slug-available", newPageHandler.CheckSlug) // ?slug=
*/

const (
	maxDisplayNameLength = 64
	maxBioLength         = 500
	maxPageBodyBytes     = 32 << 10 // whole request body incl. settings; use http.MaxBytesReader
)

func (ph *PageHandler) GetPublicPage(w http.ResponseWriter, r *http.Request) {
	const op = "internal/features/page/http GetPublicPage"
	// TODO: viewerID from jwtcontext.GetUserID (may be absent — anonymous), chi.URLParam(r, "slug"),
	// lowercase + trim; not found / unpublished (and viewer is not the owner) -> 404 (same answer for both)
	responseReturn(w, http.StatusNotImplemented, dto.PublicPageResponse{Message: "not implemented"})
}

func (ph *PageHandler) GetMyPage(w http.ResponseWriter, r *http.Request) {
	const op = "internal/features/page/http GetMyPage"
	// TODO: no page yet -> 404 (frontend shows "create page" button in view settings)
	responseReturn(w, http.StatusNotImplemented, dto.PageResponse{Message: "not implemented"})
}

func (ph *PageHandler) CreatePage(w http.ResponseWriter, r *http.Request) {
	const op = "internal/features/page/http CreatePage"
	// TODO: decode dto.CreatePageRequest; already exists -> 409, slug taken -> 409, reserved/invalid -> 400
	responseReturn(w, http.StatusNotImplemented, dto.PageResponse{Message: "not implemented"})
}

func (ph *PageHandler) UpdateMyPage(w http.ResponseWriter, r *http.Request) {
	const op = "internal/features/page/http UpdateMyPage"
	// TODO: limit body with maxPageBodyBytes, decode dto.UpdatePageRequest,
	// lengths of display_name / bio; invalid settings -> 400 with a readable reason
	responseReturn(w, http.StatusNotImplemented, dto.PageResponse{Message: "not implemented"})
}

func (ph *PageHandler) ChangeSlug(w http.ResponseWriter, r *http.Request) {
	const op = "internal/features/page/http ChangeSlug"
	// TODO: cooldown -> 429 (or 409), taken -> 409, reserved/invalid -> 400
	responseReturn(w, http.StatusNotImplemented, dto.MessageResponse{Message: "not implemented"})
}

func (ph *PageHandler) CheckSlug(w http.ResponseWriter, r *http.Request) {
	const op = "internal/features/page/http CheckSlug"
	// TODO: r.URL.Query().Get("slug")
	responseReturn(w, http.StatusNotImplemented, dto.SlugAvailabilityResponse{Message: "not implemented"})
}

func responseReturn(w http.ResponseWriter, status int, resp any, headers ...httpfeatures.Header) {
	w.Header().Set("Content-Type", "application/json")

	for _, h := range headers {
		w.Header().Set(h.Key, h.Value)
	}

	w.WriteHeader(status)
	json.NewEncoder(w).Encode(resp)
}
