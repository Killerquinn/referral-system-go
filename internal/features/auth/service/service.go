package service

import (
	"context"

	"go.uber.org/zap"
)

type Auth struct {
	log      *zap.Logger
	userAuth UserAuth
}

type UserAuth interface {
	UserExists(ctx context.Context, email string) (bool, error)
}

func New(
	log *zap.Logger,
	userAuth UserAuth) *Auth {
	return &Auth{
		log:      log,
		userAuth: userAuth,
	}
}

func (auth *Auth) RegisterUser(ctx context.Context, username string, email string, password []byte) (userid string, err error) {
	const op = "auth/service.registernewuser"
	panic("implement")
}
