package service

import (
	"context"
	"time"

	"go.uber.org/zap"
)

type RefService struct {
	log      *zap.Logger
	refCheck referralCheck
}

type referralCheck interface {
	//repository methods
}

func New(
	log *zap.Logger,
	referralCheck referralCheck,
) *RefService {
	return &RefService{
		log:      log,
		refCheck: referralCheck,
	}
}

func (rfs *RefService) WhoseReferralUserIs(ctx context.Context, username string) (referrer string, referrerSince time.Time, referrerprofileURL string, err error) {
	panic("implement me!~")
}
