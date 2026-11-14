package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/killerquinn/referral-system-go/internal/features/upload/dto"
	httpfeatures "github.com/killerquinn/referral-system-go/internal/infrastructure/pkg/http-features"
	"github.com/killerquinn/referral-system-go/internal/infrastructure/pkg/jwtcontext"
	sharederrors "github.com/killerquinn/referral-system-go/internal/shared/shared-errors.go"
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

	userID, ok := r.Context().Value(jwtcontext.UserIDKey).(string)
	if !ok {
		responseReturn(w, http.StatusUnauthorized, dto.InitUploadResponse{Message: "unauthorized"})
		return
	}

	var req dto.InitUploadRequest

	r.Body = http.MaxBytesReader(w, r.Body, maxInitBodyBytes)

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			responseReturn(w, http.StatusRequestEntityTooLarge, dto.InitUploadResponse{Message: "request entity too large"})
			return
		}
		responseReturn(w, http.StatusBadRequest, dto.InitUploadResponse{Message: "bad request"})
		return
	}

	if req.Purpose == "" || req.ContentType == "" || req.Size <= 0 {
		responseReturn(w, http.StatusBadRequest, dto.InitUploadResponse{Message: "bad request"})
		return
	}

	uploadedItem, presignedUpload, err := uh.uService.InitUpload(r.Context(), userID, req)
	if err != nil {
		status, msg := serviceErrorStatus(err)
		responseReturn(w, status, dto.InitUploadResponse{Message: msg})
	}

	responseReturn(w, http.StatusCreated, dto.InitUploadResponse{Upload: uploadedItem, Target: presignedUpload, Message: "created successfully"})
}

func (uh *UploadHandler) CompleteUpload(w http.ResponseWriter, r *http.Request) {
	userID, ok := jwtcontext.GetUserID(r.Context())
	if !ok {
		responseReturn(w, http.StatusUnauthorized, dto.UploadResponse{Message: "unauthorized"})
		return
	}

	uploadID, ok := uploadIDFromURL(r)
	if !ok {
		responseReturn(w, http.StatusNotFound, dto.UploadResponse{Message: sharederrors.ErrUploadNotFound.Error()})
		return
	}

	item, err := uh.uService.CompleteUpload(r.Context(), userID, uploadID)
	if err != nil {
		status, msg := serviceErrorStatus(err)
		responseReturn(w, status, dto.UploadResponse{Message: msg})
		return
	}

	responseReturn(w, http.StatusOK, dto.UploadResponse{Upload: &item, Message: "upload is being processed"})
}

func (uh *UploadHandler) GetUpload(w http.ResponseWriter, r *http.Request) {
	userID, ok := jwtcontext.GetUserID(r.Context())
	if !ok {
		responseReturn(w, http.StatusUnauthorized, dto.UploadResponse{Message: "unauthorized"})
		return
	}

	uploadID, ok := uploadIDFromURL(r)
	if !ok {
		responseReturn(w, http.StatusNotFound, dto.UploadResponse{Message: sharederrors.ErrUploadNotFound.Error()})
		return
	}

	item, err := uh.uService.GetUpload(r.Context(), userID, uploadID)
	if err != nil {
		status, msg := serviceErrorStatus(err)
		responseReturn(w, status, dto.UploadResponse{Message: msg})
		return
	}

	responseReturn(w, http.StatusOK, dto.UploadResponse{Upload: &item, Message: "ok"})
}

func (uh *UploadHandler) DeleteUpload(w http.ResponseWriter, r *http.Request) {
	userID, ok := jwtcontext.GetUserID(r.Context())
	if !ok {
		responseReturn(w, http.StatusUnauthorized, dto.MessageResponse{Message: "unauthorized"})
		return
	}

	uploadID, ok := uploadIDFromURL(r)
	if !ok {
		responseReturn(w, http.StatusNotFound, dto.MessageResponse{Message: sharederrors.ErrUploadNotFound.Error()})
		return
	}

	if err := uh.uService.DeleteUpload(r.Context(), userID, uploadID); err != nil {
		status, msg := serviceErrorStatus(err)
		responseReturn(w, status, dto.MessageResponse{Message: msg})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (uh *UploadHandler) GetQuota(w http.ResponseWriter, r *http.Request) {
	userID, ok := jwtcontext.GetUserID(r.Context())
	if !ok {
		responseReturn(w, http.StatusUnauthorized, dto.QuotaResponse{Message: "unauthorized"})
		return
	}

	quota, err := uh.uService.GetQuota(r.Context(), userID)
	if err != nil {
		status, msg := serviceErrorStatus(err)
		responseReturn(w, status, dto.QuotaResponse{Message: msg})
		return
	}

	responseReturn(w, http.StatusOK, dto.QuotaResponse{LimitBytes: quota.LimitBytes, UsedBytes: quota.UsedBytes, Message: "ok"})
}

func uploadIDFromURL(r *http.Request) (string, bool) {
	id := chi.URLParam(r, "uploadID")
	if _, err := uuid.Parse(id); err != nil {
		return "", false
	}
	return id, true
}

func serviceErrorStatus(err error) (int, string) {
	switch {
	case errors.Is(err, sharederrors.ErrUploadNotFound):
		return http.StatusNotFound, err.Error()
	case errors.Is(err, sharederrors.ErrUploadNotPending):
		return http.StatusConflict, err.Error()
	case errors.Is(err, sharederrors.ErrUploadIncomplete):
		return http.StatusBadRequest, err.Error()
	case errors.Is(err, sharederrors.ErrUnsupportedMediaType):
		return http.StatusUnsupportedMediaType, err.Error()
	case errors.Is(err, sharederrors.ErrFileTooLarge):
		return http.StatusRequestEntityTooLarge, err.Error()
	case errors.Is(err, sharederrors.ErrTooManyPendingUploads):
		return http.StatusTooManyRequests, err.Error()
	case errors.Is(err, sharederrors.ErrStorageQuotaExceeded):
		return http.StatusForbidden, err.Error()
	case errors.Is(err, sharederrors.ErrStorageUnavailable):
		return http.StatusServiceUnavailable, err.Error()
	default:
		return http.StatusInternalServerError, "internal server error"
	}
}

func responseReturn(w http.ResponseWriter, status int, resp any, headers ...httpfeatures.Header) {
	w.Header().Set("Content-Type", "application/json")

	for _, h := range headers {
		w.Header().Set(h.Key, h.Value)
	}

	w.WriteHeader(status)
	json.NewEncoder(w).Encode(resp)
}
