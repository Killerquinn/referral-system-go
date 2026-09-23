package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/killerquinn/referral-system-go/internal/features/referral/dto"
	httpfeatures "github.com/killerquinn/referral-system-go/internal/infrastructure/pkg/http-features"
	sharederrors "github.com/killerquinn/referral-system-go/internal/shared/shared-errors.go"
	"go.uber.org/zap"
)

type RefService struct {
	log      *zap.Logger
	baseurl  string
	refCheck referralCheck
}

type referralCheck interface {
	CheckReferralByUsername(ctx context.Context, username string) (referrerusername string, referredSince time.Time, err error)
	GetReferralsByRefferersUsername(ctx context.Context, username string, cursor string, limit int) (entities []dto.ReferralItem, err error)
	GetReferralsByOwnUserID(ctx context.Context, username string, cursor string, limit int) (entities []dto.ReferralItem, err error)
}

func New(
	log *zap.Logger,
	baseurl string,
	referralCheck referralCheck,
) *RefService {
	return &RefService{
		log:      log,
		baseurl:  baseurl,
		refCheck: referralCheck,
	}
}

func (rfs *RefService) WhoseReferralUserIs(ctx context.Context, username string) (referrer string, referrerprofileURL string, referrerSince time.Time, err error) {
	const op = "/internal/features/referral/service WhoseReferralUserIs"

	log := rfs.log.With(
		zap.String("preparing to check whose referral user is: ", username),
		zap.String("op: ", op),
	)

	refusername, lastTimeReferred, err := rfs.refCheck.CheckReferralByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, sharederrors.ErrUserNotFound) {

			return "", "", time.Time{}, sharederrors.ErrUserNotFound
		}
		log.Error("err:", zap.Error(err))

		return "", "", time.Time{}, fmt.Errorf("%s%w", op, err)
	}

	return refusername, httpfeatures.BuildUrl(rfs.baseurl, refusername), lastTimeReferred, nil
}

func (rfs *RefService) CheckCurrentReferralList(ctx context.Context, referrerID string, params dto.ReferralListParams) (*dto.CurrentReferralListResponse, error) {
	const op = "/internal/features/referral/service CheckCurrentReferralList"

	log := rfs.log.With(
		zap.String("preparing to cursor based pagination to", referrerID),
		zap.String("op: ", op),
	)

	if params.Limit <= 0 {
		params.Limit = 0
	}
	if params.Limit > 100 {
		params.Limit = 100
	}

	var (
		entities []dto.ReferralItem
		err      error
	)

	switch _, parseErr := uuid.Parse(referrerID); parseErr {
	case nil:
		log.Info("user tries to check his own referrals")
		entities, err = rfs.refCheck.GetReferralsByOwnUserID(ctx, referrerID, params.Cursor, params.Limit)
	default:
		entities, err = rfs.refCheck.GetReferralsByRefferersUsername(ctx, referrerID, params.Cursor, params.Limit)
	}

	if err != nil {
		switch {
		case errors.Is(err, sharederrors.ErrUserNotFound):
			return nil, sharederrors.ErrUserNotFound
		case errors.Is(err, sharederrors.ErrUserIsBanned):
			return nil, sharederrors.ErrUserIsBanned
		case errors.Is(err, sharederrors.ErrUserDoesntHaveReferralsYet):
			return nil, sharederrors.ErrUserDoesntHaveReferralsYet

		default:
			log.Error("failed to get referrals", zap.Error(err), zap.String("referrer_id", referrerID))
			return nil, fmt.Errorf("%s: %w", op, err)
		}
	}
	hasMore := false
	var nextCursor *time.Time

	if len(entities) > params.Limit {
		hasMore = true

		entities = entities[:params.Limit]

		lastTimeItem := entities[len(entities)].JoinedAt

		nextCursor = &lastTimeItem
	}
	items := make([]dto.ReferralItem, 0, len(entities))
	for _, e := range entities {
		items = append(items, dto.ReferralItem{
			ID:         e.ID,
			Username:   e.Username,
			ProfileURL: httpfeatures.BuildUrl(rfs.baseurl, e.Username),
			JoinedAt:   e.JoinedAt,
		})
	}

	return &dto.CurrentReferralListResponse{
		Items:      items,
		NextCursor: httpfeatures.TimePtrToStringPtr(nextCursor),
		HasMore:    hasMore,
	}, nil

}
