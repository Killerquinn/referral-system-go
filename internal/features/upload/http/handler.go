package http

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/killerquinn/referral-system-go/internal/features/upload/dto"
	httpfeatures "github.com/killerquinn/referral-system-go/internal/infrastructure/pkg/http-features"
)

type UploadService interface {
	InitUpload(ctx context.Context, userID string, req dto.InitUploadRequest) (*dto.UploadItem, *dto.PresignedUpload, error)
	CompleteUpload(ctx context.Context, userID, uploadID string) (dto.UploadItem, error)
	GetUpload(ctx context.Context, userID, uploadID string) (dto.UploadItem, error)
	DeleteUpload(ctx context.Context, userID, uploadID string) error
	GetQuota(ctx context.Context, userID string) (dto.QuotaRow, error)
}

type UploadHandler struct {
	uService UploadService
}

func NewUploadHandler(us UploadService) *UploadHandler {
	return &UploadHandler{uService: us}
}

/*
Routes (rest/chi.go), inside the AuthMiddleware group
(static "quota" wins over {uploadID} in chi):

	r.Post("/api/v1/uploads", newUploadHandler.InitUpload)
	r.Get("/api/v1/uploads/quota", newUploadHandler.GetQuota)
	r.Post("/api/v1/uploads/{uploadID}/complete", newUploadHandler.CompleteUpload)
	r.Get("/api/v1/uploads/{uploadID}", newUploadHandler.GetUpload)
	r.Delete("/api/v1/uploads/{uploadID}", newUploadHandler.DeleteUpload)

Init is a good candidate for a rate limit (presigned URLs are cheap to request, storage is not).

Error -> status:
	ErrUnsupportedMediaType  -> 415
	ErrFileTooLarge          -> 413
	ErrStorageQuotaExceeded  -> 403 (or 413)
	ErrTooManyPendingUploads -> 429
	ErrUploadNotFound        -> 404
	ErrUploadNotPending      -> 409
	ErrUploadIncomplete      -> 400 (frontend: "upload failed, try again")
	ErrStorageUnavailable    -> 503
*/

const maxInitBodyBytes = 4 << 10

func (uh *UploadHandler) InitUpload(w http.ResponseWriter, r *http.Request) {
	const op = "internal/features/upload/http InitUpload"
	// TODO: userID from jwtcontext, limit body with maxInitBodyBytes, decode dto.InitUploadRequest,
	// purpose / content_type required; 201 with upload + target
	responseReturn(w, http.StatusNotImplemented, dto.InitUploadResponse{Message: "not implemented"})
}

func (uh *UploadHandler) CompleteUpload(w http.ResponseWriter, r *http.Request) {
	const op = "internal/features/upload/http CompleteUpload"
	// TODO: chi.URLParam(r, "uploadID") must be a uuid (invalid -> 404, same as not found)
	responseReturn(w, http.StatusNotImplemented, dto.UploadResponse{Message: "not implemented"})
}

func (uh *UploadHandler) GetUpload(w http.ResponseWriter, r *http.Request) {
	const op = "internal/features/upload/http GetUpload"
	// TODO: uploadID must be a uuid
	responseReturn(w, http.StatusNotImplemented, dto.UploadResponse{Message: "not implemented"})
}

func (uh *UploadHandler) DeleteUpload(w http.ResponseWriter, r *http.Request) {
	const op = "internal/features/upload/http DeleteUpload"
	// TODO: uploadID must be a uuid; 204 on success
	responseReturn(w, http.StatusNotImplemented, dto.MessageResponse{Message: "not implemented"})
}

func (uh *UploadHandler) GetQuota(w http.ResponseWriter, r *http.Request) {
	const op = "internal/features/upload/http GetQuota"
	// TODO
	responseReturn(w, http.StatusNotImplemented, dto.QuotaResponse{Message: "not implemented"})
}

func responseReturn(w http.ResponseWriter, status int, resp any, headers ...httpfeatures.Header) {
	w.Header().Set("Content-Type", "application/json")

	for _, h := range headers {
		w.Header().Set(h.Key, h.Value)
	}

	w.WriteHeader(status)
	json.NewEncoder(w).Encode(resp)
}
