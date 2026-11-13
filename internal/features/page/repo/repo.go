package repo

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/killerquinn/referral-system-go/internal/features/page/dto"
	sharederrors "github.com/killerquinn/referral-system-go/internal/shared/shared-errors.go"
)

type Repository struct {
	db *pgxpool.Pool
}

func New(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) CreatePage(ctx context.Context, userID, slug string, displayName *string, settings dto.PageSettings) (dto.PageItem, error) {
	const op = "internal/features/page/repo CreatePage"
	// TODO: unique_violation (23505): constraint on user_id (pkey) -> sharederrors.ErrPageAlreadyExists,
	// on slug -> sharederrors.ErrSlugTaken (pgconn.PgError.ConstraintName tells which one)
	return dto.PageItem{}, sharederrors.ErrNotImplemented
}

func (r *Repository) GetPageByUserID(ctx context.Context, userID string) (dto.PageItem, error) {
	const op = "internal/features/page/repo GetPageByUserID"
	// TODO: pgx.ErrNoRows -> sharederrors.ErrPageNotFound
	return dto.PageItem{}, sharederrors.ErrNotImplemented
}

func (r *Repository) GetPageBySlug(ctx context.Context, slug string) (dto.PageWithAuthor, error) {
	const op = "internal/features/page/repo GetPageBySlug"
	// TODO: pgx.ErrNoRows -> sharederrors.ErrPageNotFound
	return dto.PageWithAuthor{}, sharederrors.ErrNotImplemented
}

func (r *Repository) UpdatePage(ctx context.Context, userID string, displayName, bio *string, settings *dto.PageSettings, isPublished *bool) (dto.PageItem, error) {
	const op = "internal/features/page/repo UpdatePage"
	// TODO: 0 rows -> sharederrors.ErrPageNotFound; "" for display_name / bio -> NULL (NULLIF in SQL)
	return dto.PageItem{}, sharederrors.ErrNotImplemented
}

func (r *Repository) ChangeSlug(ctx context.Context, userID, newSlug string) error {
	const op = "internal/features/page/repo ChangeSlug"
	// TODO: 0 rows -> sharederrors.ErrPageNotFound, unique_violation -> sharederrors.ErrSlugTaken
	return sharederrors.ErrNotImplemented
}

func (r *Repository) IsSlugTaken(ctx context.Context, slug string) (bool, error) {
	const op = "internal/features/page/repo IsSlugTaken"
	// TODO
	return false, sharederrors.ErrNotImplemented
}

func (r *Repository) CountReferrals(ctx context.Context, authorID string) (int, error) {
	const op = "internal/features/page/repo CountReferrals"
	// TODO
	return 0, sharederrors.ErrNotImplemented
}

func (r *Repository) GetRecentPostPreviews(ctx context.Context, authorID string, limit int) ([]dto.PostPreview, error) {
	const op = "internal/features/page/repo GetRecentPostPreviews"
	// TODO: empty result is not an error
	return nil, sharederrors.ErrNotImplemented
}

func (r *Repository) GetViewerContext(ctx context.Context, viewerID, authorID string) (dto.ViewerContext, error) {
	const op = "internal/features/page/repo GetViewerContext"
	// TODO: unknown viewer (deleted after token was issued) -> treat as anonymous in the service
	return dto.ViewerContext{}, sharederrors.ErrNotImplemented
}
