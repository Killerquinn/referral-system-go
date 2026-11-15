package service

import (
	"context"
	"errors"
	"fmt"
	"mime"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/killerquinn/referral-system-go/internal/features/upload/dto"
	"github.com/killerquinn/referral-system-go/internal/infrastructure/storage/r2"
	sharederrors "github.com/killerquinn/referral-system-go/internal/shared/shared-errors.go"
	"go.uber.org/zap"
)

/*
Upload flow:

 1. InitUpload     — validate purpose / type / size / quota, create a `pending` row,
                     return a presigned PUT to the PRIVATE bucket at incoming/<upload_id>.
 2. browser        — PUT the file straight to R2 (our server never touches the bytes).
 3. CompleteUpload — Head the object: exists and size == declared -> `processing`, enqueue processing.
 4. processing job — (next step, River + libvips) detect real type by content, strip EXIF,
                     make variants, move to the final bucket, `ready` + quota in one transaction.
 5. frontend polls GetUpload until ready / failed.
*/

type uploadStorage interface {
	// unique violation on incoming_key is impossible in practice (key contains the uuid)
	CreateUpload(ctx context.Context, p dto.CreateUploadParams) (dto.UploadRow, error)
	// pgx.ErrNoRows -> sharederrors.ErrUploadNotFound
	GetUpload(ctx context.Context, uploadID string) (dto.UploadRow, error)
	// only pending -> processing, only for the owner; 0 rows -> sharederrors.ErrUploadNotPending
	MarkProcessing(ctx context.Context, uploadID, ownerID string, sizeBytes int64) (dto.UploadRow, error)
	// deletes the row (and decrements the quota if it was ready) in one transaction, returns the deleted row
	DeleteUpload(ctx context.Context, uploadID, ownerID string) (dto.UploadRow, error)
	CountRecentPending(ctx context.Context, ownerID string, since time.Time) (int, error)
	// creates the quota row with defaultLimit if missing, returns it
	EnsureQuota(ctx context.Context, userID string, defaultLimit int64) (dto.QuotaRow, error)
}

// objectStorage is the subset of *r2.Client this service needs.
type objectStorage interface {
	PresignPut(ctx context.Context, bucket, key, contentType string, size int64) (*r2.PresignedUpload, error)
	Head(ctx context.Context, bucket, key string) (*r2.ObjectInfo, error)
	Delete(ctx context.Context, bucket, key string) error
	PublicURL(key string) string
	PublicBucket() string
	PrivateBucket() string
}

// processingQueue puts a "process upload" job into the background queue (River, next step).
type processingQueue interface {
	EnqueueProcessUpload(ctx context.Context, uploadID string) error
}

type UploadService struct {
	log     *zap.Logger
	storage uploadStorage
	objects objectStorage   // nil when R2 is not configured -> sharederrors.ErrStorageUnavailable
	queue   processingQueue // nil until River is set up -> uploads just stay in `processing`
}

// New: pass objects / queue as untyped nil when they're absent. A nil *r2.Client stored in the
// interface is NOT == nil (typed nil), so the nil checks below would not work.
func New(log *zap.Logger, storage uploadStorage, objects objectStorage, queue processingQueue) *UploadService {
	return &UploadService{
		log:     log,
		storage: storage,
		objects: objects,
		queue:   queue,
	}
}

// ===== policy (single source of truth, the frontend mirrors it for the file picker) =====

type mediaRule struct {
	mediaType string
	maxBytes  int64
}

const mb = 1 << 20

var (
	imageRules = func(maxBytes int64) map[string]mediaRule {
		// HEIC later: needs libvips built with libheif
		return map[string]mediaRule{
			"image/jpeg": {dto.MediaImage, maxBytes},
			"image/png":  {dto.MediaImage, maxBytes},
			"image/webp": {dto.MediaImage, maxBytes},
			"image/gif":  {dto.MediaImage, maxBytes},
		}
	}

	// purpose -> content type -> rule. Anything not listed getting rejected.
	uploadPolicy = map[string]map[string]mediaRule{
		dto.PurposeAvatar: imageRules(5 * mb),
		dto.PurposePageBackground: merge(imageRules(10*mb), map[string]mediaRule{
			// short looped background video, served from R2 as is (no transcoding)
			"video/mp4":  {dto.MediaVideo, 50 * mb},
			"video/webm": {dto.MediaVideo, 50 * mb},
		}),
		dto.PurposePageMusic: {
			"audio/mpeg": {dto.MediaAudio, 15 * mb}, // mp3
			"audio/mp4":  {dto.MediaAudio, 15 * mb}, // m4a / aac
			"audio/aac":  {dto.MediaAudio, 15 * mb},
			"audio/ogg":  {dto.MediaAudio, 15 * mb},
		},
		// video in posts comes later via Stream (provider = cf_stream)
		dto.PurposePostMedia: imageRules(20 * mb),
	}
)

const (
	defaultStorageLimit = 500 * mb
	maxPendingUploads   = 10 // per user within pendingWindow — stops spamming presigned URLs
	pendingWindow       = time.Hour
	maxFilenameLength   = 255
	incomingPrefix      = "incoming/"
)

func merge(a, b map[string]mediaRule) map[string]mediaRule {
	out := make(map[string]mediaRule, len(a)+len(b))
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		out[k] = v
	}
	return out
}

// incomingKey is where the raw file lands. Built only from our uuid — never from the client's filename.
func incomingKey(uploadID string) string {
	return incomingPrefix + uploadID
}

// ruleFor finds the rule for purpose + content type.
func ruleFor(purpose, contentType string) (mediaRule, string, error) {
	normalizedCT, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return mediaRule{}, "", sharederrors.ErrUnsupportedMediaType
	}

	rules, ok := uploadPolicy[purpose]
	if !ok {
		return mediaRule{}, "", sharederrors.ErrUnsupportedMediaType
	}

	rule, ok := rules[normalizedCT]
	if !ok {
		return mediaRule{}, "", sharederrors.ErrUnsupportedMediaType
	}

	return rule, normalizedCT, nil
}

// toItem maps a row to the API shape. URL / ThumbURL only for ready files in the public bucket
// (objects.PublicURL(storage_key / variants.thumb)); never expose incoming_key.
func (us *UploadService) toItem(row dto.UploadRow) dto.UploadItem {
	var thumbURL *string
	var pubURL *string

	if us.objects != nil &&
		row.Status == dto.StatusReady &&
		row.Bucket != nil &&
		*row.Bucket == dto.BucketPublic &&
		row.StorageKey != nil {

		url := us.objects.PublicURL(*row.StorageKey)
		pubURL = &url

		if row.Variants != nil &&
			row.Variants.Thumb != "" {
			thumbUrl := us.objects.PublicURL(row.Variants.Thumb)
			thumbURL = &thumbUrl
		}
	}

	return dto.UploadItem{
		ID:               row.ID,
		MediaType:        row.MediaType,
		Purpose:          row.Purpose,
		Status:           row.Status,
		OriginalFilename: row.OriginalFilename,
		SizeBytes:        row.SizeBytes,
		Width:            row.Width,
		Height:           row.Height,
		DurationMs:       row.DurationMs,
		Blurhash:         row.Blurhash,
		URL:              pubURL,
		ThumbURL:         thumbURL,
		Error:            row.Error,
		CreatedAt:        row.CreatedAt,
		ReadyAt:          row.ReadyAt,
	}
}

// InitUpload validates the request and returns a presigned PUT for the raw file.
func (us *UploadService) InitUpload(ctx context.Context, userID string, req dto.InitUploadRequest) (*dto.UploadItem, *dto.PresignedUpload, error) {
	const op = "internal/features/upload/service InitUpload"

	log := us.log.With(
		zap.String("op", op),
		zap.String("user_id", userID),
	)

	if us.objects == nil {
		return nil, nil, sharederrors.ErrStorageUnavailable
	}

	rule, cType, err := ruleFor(req.Purpose, req.ContentType)
	if err != nil {
		return nil, nil, err
	}

	if req.Size > rule.maxBytes {
		return nil, nil, sharederrors.ErrFileTooLarge
	}

	recentPended, err := us.storage.CountRecentPending(ctx, userID, time.Now().Add(-pendingWindow))
	if err != nil {
		log.Error("method CountRecentPending failed", zap.Error(err))

		return nil, nil, fmt.Errorf("%s:%w", op, err)
	}
	if recentPended >= maxPendingUploads {
		return nil, nil, sharederrors.ErrTooManyPendingUploads
	}

	quota, err := us.storage.EnsureQuota(ctx, userID, defaultStorageLimit)
	if err != nil {
		log.Error("method EnsureQuota failed", zap.Error(err))

		return nil, nil, fmt.Errorf("%s:%w", op, err)
	}

	if quota.UsedBytes+req.Size > quota.LimitBytes {
		return nil, nil, sharederrors.ErrStorageQuotaExceeded
	}

	id := uuid.NewString()

	var filename *string

	if name := strings.TrimSpace(req.Filename); name != "" {
		if r := []rune(name); len(r) > maxFilenameLength {
			name = string(r[:maxFilenameLength])
		}
		filename = &name
	}

	row, err := us.storage.CreateUpload(ctx, dto.CreateUploadParams{
		ID:                  id,
		OwnerID:             userID,
		Purpose:             req.Purpose,
		MediaType:           rule.mediaType,
		IncomingKey:         incomingKey(id),
		DeclaredContentType: cType,
		DeclaredSizeBytes:   req.Size,
		OriginalFilename:    filename,
	})
	if err != nil {
		log.Error("method CreateUpload failed", zap.Error(err))

		return nil, nil, fmt.Errorf("%s:%w", op, err)
	}

	presigned, err := us.objects.PresignPut(ctx, us.objects.PrivateBucket(), incomingKey(id), cType, req.Size)
	if err != nil {
		log.Error("method PresignPut failed", zap.Error(err))

		return nil, nil, fmt.Errorf("%s:%w", op, err)
	}

	item := us.toItem(row)

	return &item, &dto.PresignedUpload{
		URL:       presigned.URL,
		Method:    presigned.Method,
		Headers:   presigned.Headers,
		ExpiresAt: presigned.ExpiresAt,
	}, nil
}

// CompleteUpload is called by the frontend after the PUT to R2 succeeded.
func (us *UploadService) CompleteUpload(ctx context.Context, userID, uploadID string) (dto.UploadItem, error) {
	const op = "internal/features/upload/service CompleteUpload"

	log := us.log.With(
		zap.String("op", op),
		zap.String("user_id", userID),
		zap.String("upload_id", uploadID),
	)

	if us.objects == nil {
		return dto.UploadItem{}, sharederrors.ErrStorageUnavailable
	}

	row, err := us.storage.GetUpload(ctx, uploadID)
	if err != nil {
		if errors.Is(err, sharederrors.ErrUploadNotFound) {
			return dto.UploadItem{}, sharederrors.ErrUploadNotFound
		}
		log.Error("method GetUpload failed", zap.Error(err))

		return dto.UploadItem{}, fmt.Errorf("%s:%w", op, err)
	}

	if row.OwnerID != userID {
		return dto.UploadItem{}, sharederrors.ErrUploadNotFound
	}
	if row.Status != dto.StatusPending {
		return dto.UploadItem{}, sharederrors.ErrUploadNotPending
	}

	info, err := us.objects.Head(ctx, us.objects.PrivateBucket(), row.IncomingKey)
	if err != nil {
		if errors.Is(err, r2.ErrObjectNotFound) {
			return dto.UploadItem{}, sharederrors.ErrUploadIncomplete
		}
		log.Error("object storage failed on method Head", zap.Error(err))

		return dto.UploadItem{}, fmt.Errorf("%s:%w", op, err)
	}
	if info.Size != row.DeclaredSizeBytes {
		return dto.UploadItem{}, sharederrors.ErrUploadIncomplete
	}

	row, err = us.storage.MarkProcessing(ctx, uploadID, userID, info.Size)
	if err != nil {
		if errors.Is(err, sharederrors.ErrUploadNotPending) {
			return dto.UploadItem{}, err
		}
		log.Error("mark processing failed", zap.Error(err))
		return dto.UploadItem{}, fmt.Errorf("%s:%w", op, err)
	}

	if us.queue != nil {
		err := us.queue.EnqueueProcessUpload(ctx, uploadID)
		if err != nil {
			log.Error("queue error", zap.Error(err))
		}
	}

	return us.toItem(row), nil
}

func (us *UploadService) GetUpload(ctx context.Context, userID, uploadID string) (dto.UploadItem, error) {
	const op = "internal/features/upload/service GetUpload"

	log := us.log.With(
		zap.String("op", op),
		zap.String("user_id", userID),
		zap.String("upload_id", uploadID),
	)

	row, err := us.storage.GetUpload(ctx, uploadID)
	if err != nil {
		if errors.Is(err, sharederrors.ErrUploadNotFound) {
			return dto.UploadItem{}, err
		}
		log.Error("failed at GetUpload", zap.Error(err))

		return dto.UploadItem{}, fmt.Errorf("%s:%w", op, err)
	}

	if row.OwnerID != userID {
		return dto.UploadItem{}, sharederrors.ErrUploadNotFound
	}

	return us.toItem(row), nil
}

// DeleteUpload removes the row and the files. Later: refuse (or detach) if the upload is used
// by the page settings or attached to a post.
func (us *UploadService) DeleteUpload(ctx context.Context, userID, uploadID string) error {
	const op = "internal/features/upload/service DeleteUpload"
	// TODO:
	//  - DeleteUpload in repo (owner check in SQL) -> 0 rows -> sharederrors.ErrUploadNotFound
	//  - then delete objects: incoming_key (private), storage_key + variants (in their bucket).
	//    DB first, files second: a leftover file is just garbage, a row pointing to a missing file is a bug.
	//    Storage errors -> log, don't fail the request.

	log := us.log.With(
		zap.String("op", op),
		zap.String("user_id", userID),
		zap.String("upload_id", uploadID),
	)

	if us.objects == nil {
		return sharederrors.ErrStorageUnavailable
	}

	row, err := us.storage.DeleteUpload(ctx, uploadID, userID)
	if err != nil {
		if errors.Is(err, sharederrors.ErrUploadNotFound) {
			return sharederrors.ErrUploadNotFound
		}
		log.Error("failed at DeleteUpload", zap.Error(err))
		return fmt.Errorf("%s:%w", op, err)
	}

	return sharederrors.ErrNotImplemented
}

func (us *UploadService) GetQuota(ctx context.Context, userID string) (dto.QuotaRow, error) {
	const op = "internal/features/upload/service GetQuota"

	log := us.log.With(
		zap.String("op", op),
		zap.String("user_id", userID),
	)

	quota, err := us.storage.EnsureQuota(ctx, userID, defaultStorageLimit)
	if err != nil {
		log.Error("failed at EnsureQuota", zap.Error(err))

		return dto.QuotaRow{}, fmt.Errorf("%s:%w", op, err)
	}

	return quota, nil
}
