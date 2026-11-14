-- Every uploaded file: avatars, page backgrounds, page music, post media.
-- Lifecycle: pending (presigned URL issued) -> processing (client said "done", file verified)
--            -> ready | failed | rejected (moderation). Stale pending rows are cleaned by a background job.
CREATE TABLE uploads(
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    purpose TEXT NOT NULL CHECK (purpose IN ('avatar', 'page_background', 'page_music', 'post_media')),
    media_type TEXT NOT NULL CHECK (media_type IN ('image', 'video', 'audio')),
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'processing', 'ready', 'failed', 'rejected')),
    provider TEXT NOT NULL DEFAULT 'r2' CHECK (provider IN ('r2', 'cf_stream')),

    -- where the raw upload lands (always the private bucket, never served to anyone)
    incoming_key TEXT NOT NULL UNIQUE,
    -- final location after processing; NULL until ready
    bucket TEXT CHECK (bucket IN ('public', 'private')),
    storage_key TEXT UNIQUE,
    variants JSONB, -- images: {"thumb": "<key>", "medium": "<key>", "full": "<key>"}
    provider_asset_id TEXT UNIQUE, -- video in Stream later

    -- what the client declared at init (signed into the presigned URL, so R2 enforces them)
    declared_content_type TEXT NOT NULL,
    declared_size_bytes BIGINT NOT NULL CHECK (declared_size_bytes > 0),
    original_filename VARCHAR(255), -- display only, NEVER used in storage keys

    -- real values, filled by processing (mime is detected from the content, not trusted from the client)
    mime_type TEXT,
    size_bytes BIGINT,
    width INTEGER,
    height INTEGER,
    duration_ms INTEGER,
    blurhash VARCHAR(64),
    error TEXT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ,
    ready_at TIMESTAMPTZ,

    CHECK (status <> 'ready' OR (bucket IS NOT NULL AND storage_key IS NOT NULL))
);

CREATE INDEX idx_uploads_owner_purpose ON uploads(owner_id, purpose, created_at DESC);
-- cleanup job: stale pending / failed rows
CREATE INDEX idx_uploads_status_created ON uploads(status, created_at);

-- Per-user storage limits. Row is created lazily on the first upload (INSERT ... ON CONFLICT DO NOTHING).
-- used_bytes counts only READY files and is updated in the same transaction as uploads.status.
CREATE TABLE media_quotas(
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    storage_limit_bytes BIGINT NOT NULL CHECK (storage_limit_bytes >= 0),
    storage_used_bytes BIGINT NOT NULL DEFAULT 0 CHECK (storage_used_bytes >= 0),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
