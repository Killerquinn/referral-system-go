package dto

import "time"

// values of uploads.purpose / media_type / status (mirror the CHECK constraints in migration 000006)
const (
	PurposeAvatar         = "avatar"
	PurposePageBackground = "page_background"
	PurposePageMusic      = "page_music"
	PurposePostMedia      = "post_media"

	MediaImage = "image"
	MediaVideo = "video"
	MediaAudio = "audio"

	StatusPending    = "pending"
	StatusProcessing = "processing"
	StatusReady      = "ready"
	StatusFailed     = "failed"
	StatusRejected   = "rejected"

	BucketPublic  = "public"
	BucketPrivate = "private"
)

type (
	// ===== repo row =====

	UploadRow struct {
		ID                  string     `db:"id"`
		OwnerID             string     `db:"owner_id"`
		Purpose             string     `db:"purpose"`
		MediaType           string     `db:"media_type"`
		Status              string     `db:"status"`
		IncomingKey         string     `db:"incoming_key"`
		Bucket              *string    `db:"bucket"`
		StorageKey          *string    `db:"storage_key"`
		Variants            *Variants  `db:"variants"`
		DeclaredContentType string     `db:"declared_content_type"`
		DeclaredSizeBytes   int64      `db:"declared_size_bytes"`
		OriginalFilename    *string    `db:"original_filename"`
		SizeBytes           *int64     `db:"size_bytes"`
		Width               *int       `db:"width"`
		Height              *int       `db:"height"`
		DurationMs          *int       `db:"duration_ms"`
		Blurhash            *string    `db:"blurhash"`
		Error               *string    `db:"error"`
		CreatedAt           time.Time  `db:"created_at"`
		ReadyAt             *time.Time `db:"ready_at"`
	}

	// Variants are storage keys of processed image sizes (JSONB uploads.variants).
	Variants struct {
		Thumb  string `json:"thumb,omitempty"`
		Medium string `json:"medium,omitempty"`
		Full   string `json:"full,omitempty"`
	}

	// CreateUploadParams is what the service passes to the repo on init.
	CreateUploadParams struct {
		ID                  string // generated in the service: it's part of incoming_key
		OwnerID             string
		Purpose             string
		MediaType           string
		IncomingKey         string
		DeclaredContentType string
		DeclaredSizeBytes   int64
		OriginalFilename    *string
	}

	// MarkReadyParams is what processing writes when the file is done.
	MarkReadyParams struct {
		ID         string
		OwnerID    string
		Bucket     string    // BucketPublic / BucketPrivate
		StorageKey string    // the main file (images: the "full" variant)
		Variants   *Variants // nil for video / audio
		MimeType   string    // sniffed from the content
		SizeBytes  int64     // what is counted in the quota: sum of all stored files
		Width      *int
		Height     *int
		DurationMs *int
		Blurhash   *string
	}

	QuotaRow struct {
		LimitBytes int64 `db:"storage_limit_bytes"`
		UsedBytes  int64 `db:"storage_used_bytes"`
	}

	// ===== API =====

	// UploadItem is what the owner sees. URLs are filled only for ready files in the public bucket
	// (private files get short-lived presigned URLs from the feature that serves them, e.g. posts).
	UploadItem struct {
		ID               string     `json:"id"`
		Purpose          string     `json:"purpose"`
		MediaType        string     `json:"media_type"`
		Status           string     `json:"status"`
		OriginalFilename *string    `json:"original_filename,omitempty"`
		SizeBytes        *int64     `json:"size_bytes,omitempty"`
		Width            *int       `json:"width,omitempty"`
		Height           *int       `json:"height,omitempty"`
		DurationMs       *int       `json:"duration_ms,omitempty"`
		Blurhash         *string    `json:"blurhash,omitempty"`
		URL              *string    `json:"url,omitempty"`
		ThumbURL         *string    `json:"thumb_url,omitempty"`
		Error            *string    `json:"error,omitempty"` // human-readable reason for failed / rejected
		CreatedAt        time.Time  `json:"created_at"`
		ReadyAt          *time.Time `json:"ready_at,omitempty"`
	}

	// ===== init =====

	InitUploadRequest struct {
		Purpose     string `json:"purpose"`
		ContentType string `json:"content_type"` // file.type in the browser
		Size        int64  `json:"size"`         // file.size in the browser
		Filename    string `json:"filename"`     // display only
	}

	// PresignedUpload — the frontend does fetch(url, {method, headers, body: file}).
	PresignedUpload struct {
		URL       string            `json:"url"`
		Method    string            `json:"method"`
		Headers   map[string]string `json:"headers"`
		ExpiresAt time.Time         `json:"expires_at"`
	}

	InitUploadResponse struct {
		Upload  *UploadItem      `json:"upload,omitempty"`
		Target  *PresignedUpload `json:"target,omitempty"`
		Message string           `json:"message"`
	}

	// ===== get / complete =====

	UploadResponse struct {
		Upload  *UploadItem `json:"upload,omitempty"`
		Message string      `json:"message"`
	}

	// ===== quota =====

	QuotaResponse struct {
		LimitBytes int64  `json:"limit_bytes"`
		UsedBytes  int64  `json:"used_bytes"`
		Message    string `json:"message"`
	}

	MessageResponse struct {
		Message string `json:"message"`
	}
)
