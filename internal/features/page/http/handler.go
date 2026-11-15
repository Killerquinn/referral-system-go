package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/killerquinn/referral-system-go/internal/features/page/dto"
	localmdw "github.com/killerquinn/referral-system-go/internal/infrastructure/http/middleware"
	httpfeatures "github.com/killerquinn/referral-system-go/internal/infrastructure/pkg/http-features"
	"github.com/killerquinn/referral-system-go/internal/infrastructure/pkg/jwtcontext"
	sharederrors "github.com/killerquinn/referral-system-go/internal/shared/shared-errors.go"
)

type PageService interface {
	// viewerID == "" for an anonymous visitor
	GetPublicPage(ctx context.Context, viewerID, slug string) (*dto.PublicPage, error)
	GetMyPage(ctx context.Context, userID string) (dto.PageItem, error)
	CreatePage(ctx context.Context, userID string, req dto.CreatePageRequest) (dto.PageItem, error)
	UpdateMyPage(ctx context.Context, userID string, req dto.UpdatePageRequest) (dto.PageItem, error)
	ChangeSlug(ctx context.Context, userID string, req dto.ChangeSlugRequest) error
	CheckSlug(ctx context.Context, slug string) (dto.SlugAvailabilityResponse, error)
	RecordView(ctx context.Context, slug, viewerID, ip, userAgent string) error
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

// lengths of display_name / bio are checked in the service
const maxPageBodyBytes = 32 << 10 // whole request body incl. settings

// pageErrorStatus maps service errors to HTTP. Unknown errors -> 500 without details.
func pageErrorStatus(err error) (int, string) {
	switch {
	case errors.Is(err, sharederrors.ErrPageNotFound), errors.Is(err, sharederrors.ErrUserIsBanned):
		return http.StatusNotFound, sharederrors.ErrPageNotFound.Error()
	case errors.Is(err, sharederrors.ErrPageAlreadyExists), errors.Is(err, sharederrors.ErrSlugTaken):
		return http.StatusConflict, err.Error()
	case errors.Is(err, sharederrors.ErrInvalidSlug), errors.Is(err, sharederrors.ErrSlugReserved),
		errors.Is(err, sharederrors.ErrInvalidPageSettings), errors.Is(err, sharederrors.ErrInvalidMediaAttachment):
		return http.StatusBadRequest, err.Error() // wrapped reasons are written for people ("invalid page settings: ...")
	case errors.Is(err, sharederrors.ErrSlugChangeCooldown):
		return http.StatusTooManyRequests, err.Error()
	default:
		return http.StatusInternalServerError, "internal server error"
	}
}

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxPageBodyBytes)).Decode(v); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			responseReturn(w, http.StatusRequestEntityTooLarge, dto.MessageResponse{Message: "request is too large"})
			return false
		}
		responseReturn(w, http.StatusBadRequest, dto.MessageResponse{Message: "invalid request body"})
		return false
	}
	return true
}

// GetPublicPage works for anonymous visitors too (OptionalAuthMiddleware).
func (ph *PageHandler) GetPublicPage(w http.ResponseWriter, r *http.Request) {
	viewerID, _ := jwtcontext.GetUserID(r.Context()) // "" = anonymous

	page, err := ph.pService.GetPublicPage(r.Context(), viewerID, chi.URLParam(r, "slug"))
	if err != nil {
		status, msg := pageErrorStatus(err)
		responseReturn(w, status, dto.PublicPageResponse{Message: msg})
		return
	}
	responseReturn(w, http.StatusOK, dto.PublicPageResponse{Page: page, Message: "ok"})
}

func (ph *PageHandler) GetMyPage(w http.ResponseWriter, r *http.Request) {
	userID, ok := jwtcontext.GetUserID(r.Context())
	if !ok {
		responseReturn(w, http.StatusUnauthorized, dto.PageResponse{Message: "unauthorized"})
		return
	}

	page, err := ph.pService.GetMyPage(r.Context(), userID)
	if err != nil {
		// 404 = no page yet -> the frontend shows "create page"
		status, msg := pageErrorStatus(err)
		responseReturn(w, status, dto.PageResponse{Message: msg})
		return
	}
	responseReturn(w, http.StatusOK, dto.PageResponse{Page: &page, Message: "ok"})
}

func (ph *PageHandler) CreatePage(w http.ResponseWriter, r *http.Request) {
	userID, ok := jwtcontext.GetUserID(r.Context())
	if !ok {
		responseReturn(w, http.StatusUnauthorized, dto.PageResponse{Message: "unauthorized"})
		return
	}

	var req dto.CreatePageRequest
	if !decode(w, r, &req) {
		return
	}

	page, err := ph.pService.CreatePage(r.Context(), userID, req)
	if err != nil {
		status, msg := pageErrorStatus(err)
		responseReturn(w, status, dto.PageResponse{Message: msg})
		return
	}
	responseReturn(w, http.StatusCreated, dto.PageResponse{Page: &page, Message: "page created"})
}

func (ph *PageHandler) UpdateMyPage(w http.ResponseWriter, r *http.Request) {
	userID, ok := jwtcontext.GetUserID(r.Context())
	if !ok {
		responseReturn(w, http.StatusUnauthorized, dto.PageResponse{Message: "unauthorized"})
		return
	}

	var req dto.UpdatePageRequest
	if !decode(w, r, &req) {
		return
	}

	page, err := ph.pService.UpdateMyPage(r.Context(), userID, req)
	if err != nil {
		status, msg := pageErrorStatus(err)
		responseReturn(w, status, dto.PageResponse{Message: msg})
		return
	}
	responseReturn(w, http.StatusOK, dto.PageResponse{Page: &page, Message: "page updated"})
}

func (ph *PageHandler) ChangeSlug(w http.ResponseWriter, r *http.Request) {
	userID, ok := jwtcontext.GetUserID(r.Context())
	if !ok {
		responseReturn(w, http.StatusUnauthorized, dto.MessageResponse{Message: "unauthorized"})
		return
	}

	var req dto.ChangeSlugRequest
	if !decode(w, r, &req) {
		return
	}

	if err := ph.pService.ChangeSlug(r.Context(), userID, req); err != nil {
		status, msg := pageErrorStatus(err)
		responseReturn(w, status, dto.MessageResponse{Message: msg})
		return
	}
	responseReturn(w, http.StatusOK, dto.MessageResponse{Message: "page address changed"})
}

func (ph *PageHandler) CheckSlug(w http.ResponseWriter, r *http.Request) {
	res, err := ph.pService.CheckSlug(r.Context(), r.URL.Query().Get("slug"))
	if err != nil {
		status, msg := pageErrorStatus(err)
		responseReturn(w, status, dto.SlugAvailabilityResponse{Message: msg})
		return
	}
	responseReturn(w, http.StatusOK, res)
}

// RecordView: POST /api/v1/pages/{slug}/view, sent by the frontend once the page is on screen.
// Token optional. Always 204 for a known page (counted or not), so it reveals nothing.
func (ph *PageHandler) RecordView(w http.ResponseWriter, r *http.Request) {
	viewerID, _ := jwtcontext.GetUserID(r.Context())

	err := ph.pService.RecordView(r.Context(), chi.URLParam(r, "slug"), viewerID, localmdw.ClientIP(r), r.UserAgent())
	if err != nil {
		status, msg := pageErrorStatus(err)
		responseReturn(w, status, dto.MessageResponse{Message: msg})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func responseReturn(w http.ResponseWriter, status int, resp any, headers ...httpfeatures.Header) {
	w.Header().Set("Content-Type", "application/json")

	for _, h := range headers {
		w.Header().Set(h.Key, h.Value)
	}

	w.WriteHeader(status)
	json.NewEncoder(w).Encode(resp)
}
