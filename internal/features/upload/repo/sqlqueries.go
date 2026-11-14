package repo

// TODO: write the queries. Placeholders below only document the expected params.
const (
	// $1 id, $2 owner_id, $3 purpose, $4 media_type, $5 incoming_key,
	// $6 declared_content_type, $7 declared_size_bytes, $8 original_filename (nullable)
	// RETURNING every column of dto.UploadRow
	InsertUpload = ``

	// $1 id
	SelectUploadByID = ``

	// $1 id, $2 owner_id, $3 size_bytes — status = 'processing', updated_at = NOW()
	// WHERE status = 'pending'; RETURNING every column
	UpdateUploadToProcessing = ``

	// $1 id, $2 owner_id — RETURNING every column
	DeleteUploadByOwner = ``

	// $1 owner_id, $2 since — status = 'pending' AND created_at >= since
	CountPendingSince = ``

	// $1 user_id, $2 storage_limit_bytes — ON CONFLICT (user_id) DO NOTHING
	InsertQuotaIfMissing = ``

	// $1 user_id
	SelectQuota = ``

	// $1 user_id, $2 bytes — GREATEST(storage_used_bytes - $2, 0), updated_at = NOW()
	DecrementQuotaUsed = ``
)
