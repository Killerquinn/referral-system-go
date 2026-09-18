package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	sharederrors "github.com/killerquinn/referral-system-go/internal/shared/shared-errors.go"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

type Uservice struct {
	log        *zap.Logger
	credChange credentialsChange
	refOpts    referralOptions
}

type credentialsChange interface {
	PullOldPassword(ctx context.Context, userID uuid.UUID) (oldPass []byte, err error)
	ChangePassword(ctx context.Context, userID uuid.UUID, newPassword []byte) (err error)
}

type referralOptions interface {
	IfAbleToChangeReferrer(ctx context.Context, userID uuid.UUID) (availableafter time.Time, err error)
	ChangeCurrentReferrer(ctx context.Context, userID uuid.UUID, refcode string, newTimestamp time.Time) (err error)
	IfReferrerExist(ctx context.Context, refcode string) (err error)
}

func New(
	log *zap.Logger,
	changeCreds credentialsChange,
	referralOptions referralOptions,
) *Uservice {
	return &Uservice{
		log:        log,
		credChange: changeCreds,
		refOpts:    referralOptions,
	}
}

func (u *Uservice) CompareAndChangePassword(ctx context.Context, userID string, oldPassword string, newPassword string) (err error) {
	const op = "/internal/features/user/service.CompareAndChangePassword"

	log := u.log.With(
		zap.String("trying to change user's password, operation - ", op),
	)

	log.Info("checking if current password is right")

	useruuid, err := uuid.Parse(userID)
	if err != nil {
		log.Error("unparseable format of string was in userID, the main problem can be in it generation before parse...")

		return fmt.Errorf("%s:%w", op, sharederrors.ErrInvalidUUID)
	}

	oldpass, err := u.credChange.PullOldPassword(ctx, useruuid)
	if err != nil {
		return fmt.Errorf("%s:%w", op, err)
	}

	if err := bcrypt.CompareHashAndPassword(oldpass, []byte(oldPassword)); err != nil {
		return fmt.Errorf("Invalid credentials")
	}

	hashedNewPass, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %v", err)
	}

	if err := u.credChange.ChangePassword(ctx, useruuid, hashedNewPass); err != nil {
		return fmt.Errorf("failed to change password:%v", err)
	}

	return nil
}

func (u *Uservice) ChangeUsersCurrentReferrer(ctx context.Context, userID string, referralString string) (newTryWillBeAfter time.Duration, err error) {
	const op = "/internal/features/user/service.ChangeUsersCurrentReferrer"

	log := u.log.With(
		zap.String("trying to change user's current referrer, operation: ", op),
		zap.String("userID", userID),
	)

	log.Info("checking if user's cooldown is gone")

	useruuid, err := uuid.Parse(userID)
	if err != nil {
		log.Error("unparseable userID format", zap.Error(err))

		return 0, fmt.Errorf("%s:%w", op, err)
	}

	timestamp, err := u.refOpts.IfAbleToChangeReferrer(ctx, useruuid)
	if err != nil {

		return 0, fmt.Errorf("%s:%w", op, err)
	}

	able, remainingtowait := isOlderThenOneMonth(timestamp)
	if !able {

		return remainingtowait, sharederrors.ErrCooldownNotPassedYet
	}

	if err = u.refOpts.IfReferrerExist(ctx, referralString); err != nil {
		if errors.Is(err, sharederrors.ErrReferrerOrReferralCodeDoesntExist) {

			return 0, sharederrors.ErrReferrerOrReferralCodeDoesntExist
		}
		u.log.Error("internal server error: ", zap.Error(err))

		return 0, fmt.Errorf("%s:%w", op, err)
	}

	now := time.Now()

	if err := u.refOpts.ChangeCurrentReferrer(ctx, useruuid, referralString, now); err != nil {
		if errors.Is(err, sharederrors.ErrSelfReferred) {
			log.Error("unnable to change current referrer to user", zap.Error(err))

			return 0, sharederrors.ErrSelfReferred
		}
		if errors.Is(err, sharederrors.ErrReferrerOrReferralCodeDoesntExist) {
			log.Error("unnable to UpdateUsersReferrer, the reason must be in query, fix it", zap.Error(err))

			return 0, sharederrors.ErrReferrerOrReferralCodeDoesntExist
		}

		log.Error("unnable somewhy change referrer to user", zap.Error(err))

		return 0, fmt.Errorf("%s:%w", op, err)
	}

	nextCoolDown := now.AddDate(0, 1, 0).Sub(now)
	return nextCoolDown, nil
}

func isOlderThenOneMonth(timestamp time.Time) (bool, time.Duration) {
	oneMonthAfter := timestamp.AddDate(0, 1, 0)
	remaining := time.Until(oneMonthAfter)

	if remaining > 0 {
		return false, remaining
	}

	return true, 0
}
