package repo

import (
	"context"
	"time"

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

func (r *Repository) CreateUpload(ctx context.Context, p dto.CreateUploadParams) (dto.UploadRow, error) {
	const op = "internal/features/upload/repo CreateUpload"
	// TODO: InsertUpload ... RETURNING all columns (pgx.CollectOneRow + pgx.RowToStructByName works with the db tags)
	return dto.UploadRow{}, sharederrors.ErrNotImplemented
}

func (r *Repository) GetUpload(ctx context.Context, uploadID string) (dto.UploadRow, error) {
	const op = "internal/features/upload/repo GetUpload"
	// TODO: pgx.ErrNoRows -> sharederrors.ErrUploadNotFound
	return dto.UploadRow{}, sharederrors.ErrNotImplemented
}

func (r *Repository) MarkProcessing(ctx context.Context, uploadID, ownerID string, sizeBytes int64) (dto.UploadRow, error) {
	const op = "internal/features/upload/repo MarkProcessing"
	// TODO: UPDATE ... WHERE id AND owner_id AND status = 'pending' RETURNING ...
	// pgx.ErrNoRows -> sharederrors.ErrUploadNotPending (two parallel "complete" calls: only one wins)
	return dto.UploadRow{}, sharederrors.ErrNotImplemented
}

func (r *Repository) DeleteUpload(ctx context.Context, uploadID, ownerID string) (dto.UploadRow, error) {
	const op = "internal/features/upload/repo DeleteUpload"
	// TODO: transaction:
	//  1. DeleteUploadByOwner ... RETURNING -> pgx.ErrNoRows -> sharederrors.ErrUploadNotFound
	//  2. if the deleted row was ready -> DecrementQuotaUsed(size_bytes)
	return dto.UploadRow{}, sharederrors.ErrNotImplemented
}

func (r *Repository) CountRecentPending(ctx context.Context, ownerID string, since time.Time) (int, error) {
	const op = "internal/features/upload/repo CountRecentPending"
	// TODO
	return 0, sharederrors.ErrNotImplemented
}

func (r *Repository) EnsureQuota(ctx context.Context, userID string, defaultLimit int64) (dto.QuotaRow, error) {
	const op = "internal/features/upload/repo EnsureQuota"
	// TODO: InsertQuotaIfMissing, then SelectQuota (or one CTE query)
	return dto.QuotaRow{}, sharederrors.ErrNotImplemented
}
