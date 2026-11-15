package repo

// uploadColumns must match dto.UploadRow field-for-field (db tags): pgx.RowToStructByName
// fails at runtime on any extra or missing column. provider / provider_asset_id / mime_type /
// updated_at are intentionally not here — the struct has no fields for them.
const uploadColumns = `id, owner_id, purpose, media_type, status, incoming_key,
	bucket, storage_key, variants,
	declared_content_type, declared_size_bytes, original_filename,
	size_bytes, width, height, duration_ms, blurhash, error,
	created_at, ready_at`

const (
	// $1 id, $2 owner_id, $3 purpose, $4 media_type, $5 incoming_key,
	// $6 declared_content_type, $7 declared_size_bytes, $8 original_filename (nullable)
	InsertUpload = `
	INSERT INTO uploads (id, owner_id, purpose, media_type, incoming_key,
		declared_content_type, declared_size_bytes, original_filename)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	RETURNING ` + uploadColumns

	// $1 id
	SelectUploadByID = `
	SELECT ` + uploadColumns + `
	FROM uploads
	WHERE id = $1`

	// $1 id, $2 owner_id, $3 size_bytes.
	// status = 'pending' in WHERE: two parallel "complete" calls -> only one gets a row back.
	UpdateUploadToProcessing = `
	UPDATE uploads
	SET status = 'processing', size_bytes = $3, updated_at = NOW()
	WHERE id = $1 AND owner_id = $2 AND status = 'pending'
	RETURNING ` + uploadColumns

	// $1 id, $2 owner_id
	DeleteUploadByOwner = `
	DELETE FROM uploads
	WHERE id = $1 AND owner_id = $2
	RETURNING ` + uploadColumns

	// $1 owner_id, $2 since
	CountPendingSince = `
	SELECT COUNT(*)
	FROM uploads
	WHERE owner_id = $1 AND status = 'pending' AND created_at >= $2`

	// $1 user_id, $2 storage_limit_bytes
	InsertQuotaIfMissing = `
	INSERT INTO media_quotas (user_id, storage_limit_bytes)
	VALUES ($1, $2)
	ON CONFLICT (user_id) DO NOTHING`

	// $1 user_id
	SelectQuota = `
	SELECT storage_limit_bytes, storage_used_bytes
	FROM media_quotas
	WHERE user_id = $1`

	// $1 user_id, $2 bytes. GREATEST keeps CHECK (storage_used_bytes >= 0) from failing
	// if the counter ever drifted below the real usage.
	DecrementQuotaUsed = `
	UPDATE media_quotas
	SET storage_used_bytes = GREATEST(storage_used_bytes - $2, 0), updated_at = NOW()
	WHERE user_id = $1`

	// $1 pending created before, $2 failed/rejected created before, $3 batch size.
	// DELETE has no LIMIT in Postgres -> pick ids in a subquery. SKIP LOCKED: two server
	// instances running cleanup at once take different rows instead of waiting on each other.
	// Never matches 'ready' -> quota is never affected.
	DeleteStaleUploads = `
	DELETE FROM uploads
	WHERE id IN (
		SELECT id FROM uploads
		WHERE (status = 'pending' AND created_at < $1)
		   OR (status IN ('failed', 'rejected') AND created_at < $2)
		ORDER BY created_at
		LIMIT $3
		FOR UPDATE SKIP LOCKED
	)
	RETURNING ` + uploadColumns

	// $1 user_id, $2 bytes. Atomic limit check: two parallel uploads can't both squeeze in.
	// No row returned -> over the limit (the row always exists: EnsureQuota ran at init).
	IncrementQuotaUsed = `
	UPDATE media_quotas
	SET storage_used_bytes = storage_used_bytes + $2, updated_at = NOW()
	WHERE user_id = $1 AND storage_used_bytes + $2 <= storage_limit_bytes`

	// $1 id, $2 bucket, $3 storage_key, $4 variants (JSONB, nullable), $5 mime_type, $6 size_bytes,
	// $7 width, $8 height, $9 duration_ms, $10 blurhash — status = 'ready', ready_at / updated_at = NOW()
	// WHERE status = 'processing'
	UpdateUploadToReady = `
	UPDATE uploads
	SET status = 'ready', bucket = $2, storage_key = $3, variants = $4, mime_type = $5,
		size_bytes = $6, width = $7, height = $8, duration_ms = $9, blurhash = $10,
		error = NULL, ready_at = NOW(), updated_at = NOW()
	WHERE id = $1 AND status = 'processing'`

	// $1 id, $2 status ('failed' / 'rejected'), $3 error — updated_at = NOW(), WHERE status = 'processing'
	UpdateUploadToFailed = `
	UPDATE uploads
	SET status = $2, error = $3, updated_at = NOW()
	WHERE id = $1 AND status = 'processing'`
)
