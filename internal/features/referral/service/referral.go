package service

import (
	"context"
	"errors"
	"fmt"
	"time"

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
