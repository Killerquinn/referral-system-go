package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/killerquinn/referral-system-go/internal/features/upload/dto"
	"github.com/killerquinn/referral-system-go/internal/infrastructure/imaging"
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
	// deletes up to limit stale pending / failed / rejected rows, returns them (never ready ones)
	DeleteStale(ctx context.Context, pendingBefore, failedBefore time.Time, limit int) ([]dto.UploadRow, error)
	// processing -> ready + quota in ONE transaction; row not in processing anymore ->
	// sharederrors.ErrUploadNotPending; quota full -> sharederrors.ErrStorageQuotaExceeded (both: nothing changed)
	MarkReady(ctx context.Context, p dto.MarkReadyParams) error
	// processing -> failed / rejected with a human-readable reason
	MarkFailed(ctx context.Context, uploadID, status, reason string) error
}

// objectStorage is the subset of *r2.Client this service needs.
type objectStorage interface {
	PresignPut(ctx context.Context, bucket, key, contentType string, size int64) (*r2.PresignedUpload, error)
	Head(ctx context.Context, bucket, key string) (*r2.ObjectInfo, error)
	Delete(ctx context.Context, bucket, key string) error
	// Get: caller closes the body. r2.ErrObjectNotFound if missing.
	Get(ctx context.Context, bucket, key string) (io.ReadCloser, *r2.ObjectInfo, error)
	Put(ctx context.Context, bucket, key, contentType string, body io.Reader, size int64) error
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

	// cleanup: presigned PUT lives 15m, so a pending row older than an hour will never be completed
	stalePendingAge = time.Hour
	staleFailedAge  = 24 * time.Hour // give the frontend a day to show the error
	cleanupBatch    = 100

	// processed files: media/<upload_id>/<name>
	mediaPrefix = "media/"
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
func (us *UploadService) bucketName(bucket string) string {
	if bucket == dto.BucketPublic {
		return us.objects.PublicBucket()
	}
	return us.objects.PrivateBucket()
}

type object struct {
	bucket string
	key    string
}

// objectsOf lists every file a row may point to: the raw incoming file (always, may already be gone)
// plus the final file and its variants once processing has produced them.
func (us *UploadService) objectsOf(row dto.UploadRow) []object {
	toDelete := []object{{bucket: us.objects.PrivateBucket(), key: row.IncomingKey}}

	if row.Bucket != nil {
		bucket := us.bucketName(*row.Bucket)
		if row.StorageKey != nil {
			toDelete = append(toDelete, object{bucket: bucket, key: *row.StorageKey})
		}
		if row.Variants != nil {
			for _, k := range []string{row.Variants.Thumb, row.Variants.Medium, row.Variants.Full} {
				if k == "" {
					continue
				}
				toDelete = append(toDelete, object{bucket: bucket, key: k})
			}
		}
	}

	return toDelete
}

// deleteObjects removes the row's files. Called after the row is gone from the DB, so failures
// are only logged: a leftover file is garbage (the R2 lifecycle rule on incoming/ is the safety net).
func (us *UploadService) deleteObjects(ctx context.Context, log *zap.Logger, row dto.UploadRow) {
	for _, o := range us.objectsOf(row) {
		if err := us.objects.Delete(ctx, o.bucket, o.key); err != nil {
			log.Warn("unable to delete object",
				zap.String("upload_id", row.ID),
				zap.String("bucket", o.bucket),
				zap.String("key", o.key),
				zap.Error(err),
			)
		}
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

	us.deleteObjects(context.WithoutCancel(ctx), log, row)

	return nil
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

// CleanupStale deletes abandoned pending uploads and old failed / rejected ones, batch by batch.
// processing rows are left alone: the queue (River) will own retries for them.
func (us *UploadService) CleanupStale(ctx context.Context) (int, error) {
	const op = "internal/features/upload/service CleanupStale"

	log := us.log.With(zap.String("op", op))

	// without R2 the rows could go but the files would stay forever -> wait until storage is back
	if us.objects == nil {
		return 0, nil
	}

	// rows are already deleted when we get to the files, so file deletes must not be cut short by shutdown
	delCtx := context.WithoutCancel(ctx)

	total := 0
	for {
		now := time.Now()
		rows, err := us.storage.DeleteStale(ctx, now.Add(-stalePendingAge), now.Add(-staleFailedAge), cleanupBatch)
		if err != nil {
			return total, fmt.Errorf("%s:%w", op, err)
		}

		for _, row := range rows {
			us.deleteObjects(delCtx, log, row)
		}
		total += len(rows)

		// a short batch means nothing is left
		if len(rows) < cleanupBatch {
			return total, nil
		}
		// shutting down / run limit hit -> continue on the next tick
		if ctx.Err() != nil {
			return total, nil
		}
	}
}

// file extensions for video / audio stored as is (images always become .webp)
var fileExt = map[string]string{
	"video/mp4":  ".mp4",
	"video/webm": ".webm",
	"audio/mpeg": ".mp3",
	"audio/mp4":  ".m4a",
	"audio/aac":  ".aac",
	"audio/ogg":  ".ogg",
}

// ProcessUpload is run by the queue worker (process_upload job) after CompleteUpload.
//
// Error contract with the queue: returning an error = "temporary problem, retry later" (R2 / DB down).
// A bad file is permanent: it is marked failed / rejected and nil is returned, so it isn't retried.
func (us *UploadService) ProcessUpload(ctx context.Context, uploadID string, attempt, maxAttempts int) (err error) {
	const op = "internal/features/upload/service ProcessUpload"

	log := us.log.With(
		zap.String("op", op),
		zap.String("upload_id", uploadID),
		zap.Int("attempt", attempt),
	)

	if us.objects == nil {
		return sharederrors.ErrStorageUnavailable
	}

	// last attempt failed with a temporary error -> River gives up, so don't leave the row in processing forever
	defer func() {
		if err != nil && attempt >= maxAttempts {
			log.Error("processing gave up", zap.Error(err))
			if mErr := us.storage.MarkFailed(context.WithoutCancel(ctx), uploadID, dto.StatusFailed,
				"processing failed, please upload the file again"); mErr != nil {
				log.Error("mark failed failed", zap.Error(mErr))
			}
		}
	}()

	// ----- 1. preconditions

	row, err := us.storage.GetUpload(ctx, uploadID)
	if err != nil {
		if errors.Is(err, sharederrors.ErrUploadNotFound) {
			return nil // deleted while waiting in the queue
		}
		return fmt.Errorf("%s:%w", op, err)
	}
	if row.Status != dto.StatusProcessing {
		return nil // already handled (e.g. a retry after a crash right after MarkReady)
	}

	// ----- 2. download + check the REAL type (the declared one is just what the browser said)

	body, _, err := us.objects.Get(ctx, us.objects.PrivateBucket(), row.IncomingKey)
	if err != nil {
		if errors.Is(err, r2.ErrObjectNotFound) {
			return us.fail(ctx, log, row.ID, dto.StatusFailed, "uploaded file is missing")
		}
		return fmt.Errorf("%s:%w", op, err)
	}
	defer body.Close()

	realType, reader, err := imaging.Sniff(body)
	if err != nil {
		return fmt.Errorf("%s:%w", op, err)
	}

	// the sniffed type must be allowed for this purpose AND be the same kind the client declared.
	// Also the only gate in front of libvips: nothing but jpeg/png/webp/gif ever reaches it.
	rule, _, err := ruleFor(row.Purpose, realType)
	if err != nil || rule.mediaType != row.MediaType {
		log.Info("content doesn't match", zap.String("declared", row.DeclaredContentType), zap.String("real", realType))
		return us.fail(ctx, log, row.ID, dto.StatusFailed, "file content doesn't match an allowed type")
	}

	// size is signed into the presigned PUT and checked in CompleteUpload; the limit is a second fence
	data, err := io.ReadAll(io.LimitReader(reader, rule.maxBytes+1))
	if err != nil {
		return fmt.Errorf("%s:%w", op, err)
	}
	if int64(len(data)) > rule.maxBytes {
		return us.fail(ctx, log, row.ID, dto.StatusFailed, "file is too large")
	}

	// ----- 3. process + store

	bucketRole := dto.BucketPublic
	if row.Purpose == dto.PurposePostMedia {
		bucketRole = dto.BucketPrivate // paid post media: served only via presigned GET after an access check
	}
	bucket := us.bucketName(bucketRole)
	prefix := mediaPrefix + row.ID + "/"

	// everything we Put is removed again unless the upload ends up ready
	var written []string
	ready := false
	defer func() {
		if ready {
			return
		}
		delCtx := context.WithoutCancel(ctx)
		for _, key := range written {
			if dErr := us.objects.Delete(delCtx, bucket, key); dErr != nil {
				log.Warn("unable to delete object", zap.String("key", key), zap.Error(dErr))
			}
		}
	}()

	put := func(key, contentType string, b []byte) error {
		if err := us.objects.Put(ctx, bucket, key, contentType, bytes.NewReader(b), int64(len(b))); err != nil {
			return err
		}
		written = append(written, key)
		return nil
	}

	params := dto.MarkReadyParams{
		ID:       row.ID,
		OwnerID:  row.OwnerID,
		Bucket:   bucketRole,
		MimeType: realType,
	}

	if rule.mediaType == dto.MediaImage {
		res, err := imaging.Process(data)
		if err != nil {
			if errors.Is(err, imaging.ErrInvalidImage) {
				log.Info("invalid image", zap.Error(err))
				return us.fail(ctx, log, row.ID, dto.StatusFailed, "image could not be read")
			}
			return fmt.Errorf("%s:%w", op, err)
		}

		variants := &dto.Variants{}
		for _, v := range res.Variants {
			key := prefix + v.Name + ".webp"
			if err := put(key, "image/webp", v.Data); err != nil {
				return fmt.Errorf("%s:%w", op, err)
			}
			params.SizeBytes += int64(len(v.Data))

			switch v.Name {
			case "thumb":
				variants.Thumb = key
			case "medium":
				variants.Medium = key
			case "full":
				variants.Full = key
			}
		}

		params.StorageKey = variants.Full
		params.Variants = variants
		params.Width = &res.Width
		params.Height = &res.Height
		params.Blurhash = &res.Blurhash
	} else {
		// video / audio: stored as is (duration needs ffprobe — later)
		key := prefix + "file" + fileExt[realType]
		if err := put(key, realType, data); err != nil {
			return fmt.Errorf("%s:%w", op, err)
		}
		params.StorageKey = key
		params.SizeBytes = int64(len(data))
	}

	// ----- 4. ready + quota (one transaction in the repo)

	if err := us.storage.MarkReady(ctx, params); err != nil {
		switch {
		case errors.Is(err, sharederrors.ErrStorageQuotaExceeded):
			return us.fail(ctx, log, row.ID, dto.StatusRejected, "storage quota exceeded")
		case errors.Is(err, sharederrors.ErrUploadNotPending):
			return nil // deleted while we were working; the deferred cleanup removes our files
		default:
			return fmt.Errorf("%s:%w", op, err)
		}
	}
	ready = true

	// ----- 5. the raw file isn't needed anymore (if this fails, the cleanup / R2 lifecycle rule gets it)

	if err := us.objects.Delete(context.WithoutCancel(ctx), us.objects.PrivateBucket(), row.IncomingKey); err != nil {
		log.Warn("unable to delete incoming file", zap.Error(err))
	}

	log.Info("upload ready", zap.String("type", realType), zap.Int64("stored_bytes", params.SizeBytes))
	return nil
}

// fail marks the upload failed / rejected. nil = handled (no retry); an error only if the DB write failed.
func (us *UploadService) fail(ctx context.Context, log *zap.Logger, uploadID, status, reason string) error {
	if err := us.storage.MarkFailed(ctx, uploadID, status, reason); err != nil {
		return fmt.Errorf("mark %s: %w", status, err)
	}
	log.Info("upload "+status, zap.String("reason", reason))
	return nil
}
