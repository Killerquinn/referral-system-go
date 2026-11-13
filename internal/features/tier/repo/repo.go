package repo

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/killerquinn/referral-system-go/internal/features/tier/dto"
	sharederrors "github.com/killerquinn/referral-system-go/internal/shared/shared-errors.go"
)

type Repository struct {
	db *pgxpool.Pool
}

func New(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) CreateTier(ctx context.Context, authorID, name string, description *string, priceAmount int, currency string) (dto.TierItem, error) {
	const op = "internal/features/tier/repo CreateTier"
	// TODO: two concurrent creates can compute the same level -> unique_violation (23505) on (author_id, level):
	// retry once or return a conflict error
	return dto.TierItem{}, sharederrors.ErrNotImplemented
}

func (r *Repository) CountActiveTiers(ctx context.Context, authorID string) (int, error) {
	const op = "internal/features/tier/repo CountActiveTiers"
	// TODO
	return 0, sharederrors.ErrNotImplemented
}

func (r *Repository) UpdateTier(ctx context.Context, tierID, authorID string, name, description *string) error {
	const op = "internal/features/tier/repo UpdateTier"
	// TODO: 0 rows affected -> sharederrors.ErrTierNotFound
	return sharederrors.ErrNotImplemented
}

func (r *Repository) ArchiveTier(ctx context.Context, tierID, authorID string) error {
	const op = "internal/features/tier/repo ArchiveTier"
	// TODO: 0 rows affected -> sharederrors.ErrTierNotFound
	return sharederrors.ErrNotImplemented
}

func (r *Repository) GetTiersByAuthorID(ctx context.Context, authorID string, includeArchived bool) ([]dto.TierItem, error) {
	const op = "internal/features/tier/repo GetTiersByAuthorID"
	// TODO: empty result is not an error
	return nil, sharederrors.ErrNotImplemented
}

func (r *Repository) GetTiersByAuthorUsername(ctx context.Context, username string) ([]dto.TierItem, error) {
	const op = "internal/features/tier/repo GetTiersByAuthorUsername"
	// TODO: check author first (SelectAuthorBlockedByUsername): no rows -> sharederrors.ErrUserNotFound,
	// blocked -> sharederrors.ErrUserIsBanned
	return nil, sharederrors.ErrNotImplemented
}
