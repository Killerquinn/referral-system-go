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
