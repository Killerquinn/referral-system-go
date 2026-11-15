package repo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/killerquinn/referral-system-go/internal/features/upload/dto"
	sharederrors "github.com/killerquinn/referral-system-go/internal/shared/shared-errors.go"
)

type Repository struct {
	db *pgxpool.Pool
}

func New(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// collectUpload reads exactly one uploads row; pgx.ErrNoRows is returned as is
// so every caller maps it to its own domain error.
func collectUpload(rows pgx.Rows) (dto.UploadRow, error) {
	return pgx.CollectOneRow(rows, pgx.RowToStructByName[dto.UploadRow])
}

func (r *Repository) CreateUpload(ctx context.Context, p dto.CreateUploadParams) (dto.UploadRow, error) {
	const op = "internal/features/upload/repo CreateUpload"

	rows, err := r.db.Query(ctx, InsertUpload,
		p.ID,
		p.OwnerID,
		p.Purpose,
		p.MediaType,
		p.IncomingKey,
		p.DeclaredContentType,
		p.DeclaredSizeBytes,
		p.OriginalFilename,
	)
	if err != nil {
		return dto.UploadRow{}, fmt.Errorf("%s:%w", op, err)
	}

	row, err := collectUpload(rows)
	if err != nil {
		return dto.UploadRow{}, fmt.Errorf("%s:%w", op, err)
	}

	return row, nil
}

func (r *Repository) GetUpload(ctx context.Context, uploadID string) (dto.UploadRow, error) {
	const op = "internal/features/upload/repo GetUpload"

	rows, err := r.db.Query(ctx, SelectUploadByID, uploadID)
	if err != nil {
		return dto.UploadRow{}, fmt.Errorf("%s:%w", op, err)
	}

	row, err := collectUpload(rows)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return dto.UploadRow{}, sharederrors.ErrUploadNotFound
		}
		return dto.UploadRow{}, fmt.Errorf("%s:%w", op, err)
	}

	return row, nil
}

func (r *Repository) MarkProcessing(ctx context.Context, uploadID, ownerID string, sizeBytes int64) (dto.UploadRow, error) {
	const op = "internal/features/upload/repo MarkProcessing"

	rows, err := r.db.Query(ctx, UpdateUploadToProcessing, uploadID, ownerID, sizeBytes)
	if err != nil {
		return dto.UploadRow{}, fmt.Errorf("%s:%w", op, err)
	}

	row, err := collectUpload(rows)
	if err != nil {
		// not pending anymore (or not ours) -> WHERE matched nothing
		if errors.Is(err, pgx.ErrNoRows) {
			return dto.UploadRow{}, sharederrors.ErrUploadNotPending
		}
		return dto.UploadRow{}, fmt.Errorf("%s:%w", op, err)
	}

	return row, nil
}

func (r *Repository) DeleteUpload(ctx context.Context, uploadID, ownerID string) (dto.UploadRow, error) {
	const op = "internal/features/upload/repo DeleteUpload"

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return dto.UploadRow{}, fmt.Errorf("%s:%w", op, err)
	}
	// no-op after a successful Commit
	defer tx.Rollback(ctx)

	rows, err := tx.Query(ctx, DeleteUploadByOwner, uploadID, ownerID)
	if err != nil {
		return dto.UploadRow{}, fmt.Errorf("%s:%w", op, err)
	}

	row, err := collectUpload(rows)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return dto.UploadRow{}, sharederrors.ErrUploadNotFound
		}
		return dto.UploadRow{}, fmt.Errorf("%s:%w", op, err)
	}

	// storage_used_bytes counts only ready files
	if row.Status == dto.StatusReady && row.SizeBytes != nil {
		if _, err := tx.Exec(ctx, DecrementQuotaUsed, ownerID, *row.SizeBytes); err != nil {
			return dto.UploadRow{}, fmt.Errorf("%s:%w", op, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return dto.UploadRow{}, fmt.Errorf("%s:%w", op, err)
	}

	return row, nil
}

func (r *Repository) CountRecentPending(ctx context.Context, ownerID string, since time.Time) (int, error) {
	const op = "internal/features/upload/repo CountRecentPending"

	var count int
	if err := r.db.QueryRow(ctx, CountPendingSince, ownerID, since).Scan(&count); err != nil {
		return 0, fmt.Errorf("%s:%w", op, err)
	}

	return count, nil
}

func (r *Repository) EnsureQuota(ctx context.Context, userID string, defaultLimit int64) (dto.QuotaRow, error) {
	const op = "internal/features/upload/repo EnsureQuota"

	if _, err := r.db.Exec(ctx, InsertQuotaIfMissing, userID, defaultLimit); err != nil {
		return dto.QuotaRow{}, fmt.Errorf("%s:%w", op, err)
	}

	rows, err := r.db.Query(ctx, SelectQuota, userID)
	if err != nil {
		return dto.QuotaRow{}, fmt.Errorf("%s:%w", op, err)
	}

	quota, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[dto.QuotaRow])
	if err != nil {
		return dto.QuotaRow{}, fmt.Errorf("%s:%w", op, err)
	}

	return quota, nil
}

func (r *Repository) DeleteStale(ctx context.Context, pendingBefore, failedBefore time.Time, limit int) ([]dto.UploadRow, error) {
	const op = "internal/features/upload/repo DeleteStale"

	rows, err := r.db.Query(ctx, DeleteStaleUploads, pendingBefore, failedBefore, limit)
	if err != nil {
		return nil, fmt.Errorf("%s:%w", op, err)
	}

	// nothing to delete -> empty slice, no ErrNoRows
	deleted, err := pgx.CollectRows(rows, pgx.RowToStructByName[dto.UploadRow])
	if err != nil {
		return nil, fmt.Errorf("%s:%w", op, err)
	}

	return deleted, nil
}

func (r *Repository) MarkReady(ctx context.Context, p dto.MarkReadyParams) error {
	const op = "internal/features/upload/repo MarkReady"

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("%s:%w", op, err)
	}
	defer tx.Rollback(ctx)

	// 1. the row: only from processing (deleted / already handled -> 0 rows)
	res, err := tx.Exec(ctx, UpdateUploadToReady,
		p.ID, p.Bucket, p.StorageKey, p.Variants, p.MimeType,
		p.SizeBytes, p.Width, p.Height, p.DurationMs, p.Blurhash,
	)
	if err != nil {
		return fmt.Errorf("%s:%w", op, err)
	}
	if res.RowsAffected() == 0 {
		return sharederrors.ErrUploadNotPending
	}

	// 2. the quota: limit is checked in the WHERE, so parallel uploads can't both squeeze in.
	// 0 rows -> over the limit -> return without Commit: the deferred Rollback undoes step 1 too.
	res, err = tx.Exec(ctx, IncrementQuotaUsed, p.OwnerID, p.SizeBytes)
	if err != nil {
		return fmt.Errorf("%s:%w", op, err)
	}
	if res.RowsAffected() == 0 {
		return sharederrors.ErrStorageQuotaExceeded
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("%s:%w", op, err)
	}
	return nil
}

func (r *Repository) MarkFailed(ctx context.Context, uploadID, status, reason string) error {
	const op = "internal/features/upload/repo MarkFailed"

	// 0 rows is fine: the upload was deleted or already finished meanwhile
	if _, err := r.db.Exec(ctx, UpdateUploadToFailed, uploadID, status, reason); err != nil {
		return fmt.Errorf("%s:%w", op, err)
	}
	return nil
}
