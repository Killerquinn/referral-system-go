package service

import (
	"context"

	"github.com/killerquinn/referral-system-go/internal/features/tier/dto"
	sharederrors "github.com/killerquinn/referral-system-go/internal/shared/shared-errors.go"
	"go.uber.org/zap"
)

type tierStorage interface {
	CreateTier(ctx context.Context, authorID, name string, description *string, priceAmount int, currency string) (dto.TierItem, error)
	CountActiveTiers(ctx context.Context, authorID string) (int, error)
	UpdateTier(ctx context.Context, tierID, authorID string, name, description *string) error
	ArchiveTier(ctx context.Context, tierID, authorID string) error
	GetTiersByAuthorID(ctx context.Context, authorID string, includeArchived bool) ([]dto.TierItem, error)
	GetTiersByAuthorUsername(ctx context.Context, username string) ([]dto.TierItem, error)
}

type TierService struct {
	log      *zap.Logger
	maxTiers int
	storage  tierStorage
}

func New(log *zap.Logger, maxTiers int, storage tierStorage) *TierService {
	return &TierService{
		log:      log,
		maxTiers: maxTiers,
		storage:  storage,
	}
}

// CreateTier adds a tier at the end of the author's ladder (level = max + 1).
func (ts *TierService) CreateTier(ctx context.Context, authorID string, req dto.CreateTierRequest) (dto.TierItem, error) {
	const op = "internal/features/tier/service CreateTier"
	// TODO: CountActiveTiers >= maxTiers -> sharederrors.ErrTierLimitReached
	return dto.TierItem{}, sharederrors.ErrNotImplemented
}

func (ts *TierService) UpdateTier(ctx context.Context, authorID, tierID string, req dto.UpdateTierRequest) error {
	const op = "internal/features/tier/service UpdateTier"
	// TODO: not found / not the author's tier -> sharederrors.ErrTierNotFound
	return sharederrors.ErrNotImplemented
}

// ArchiveTier hides a tier from new subscribers; posts and subscriptions keep pointing at it.
func (ts *TierService) ArchiveTier(ctx context.Context, authorID, tierID string) error {
	const op = "internal/features/tier/service ArchiveTier"
	// TODO: not found / not the author's tier -> sharederrors.ErrTierNotFound
	return sharederrors.ErrNotImplemented
}

// TiersOfUser returns tiers of a user; empty username = caller's own tiers (archived included).
// Tiers of other users are public (they will be shown on the author's page), no referral check.
func (ts *TierService) TiersOfUser(ctx context.Context, callerID, username string) (*dto.TierListResponse, error) {
	const op = "internal/features/tier/service TiersOfUser"
	// TODO: banned author -> sharederrors.ErrUserIsBanned, unknown -> sharederrors.ErrUserNotFound
	return nil, sharederrors.ErrNotImplemented
}
