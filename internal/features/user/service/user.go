package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	sharederrors "github.com/killerquinn/referral-system-go/internal/shared/shared-errors.go"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

type Uservice struct {
	log        *zap.Logger
	credChange credentialsChange
}

type credentialsChange interface {
	PullOldPassword(ctx context.Context, userID uuid.UUID) (oldPass []byte, err error)
	ChangePassword(ctx context.Context, userID uuid.UUID, newPassword []byte) (err error)
}

func New(
	log *zap.Logger,
	changeCreds credentialsChange,
) *Uservice {
	return &Uservice{
		log:        log,
		credChange: changeCreds,
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
