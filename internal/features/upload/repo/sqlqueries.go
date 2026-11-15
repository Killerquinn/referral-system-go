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
)
