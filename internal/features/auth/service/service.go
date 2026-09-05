package service

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

type Auth struct {
	log      *zap.Logger
	userAuth userAuth
	newUser  newUser
}

type userAuth interface {
	UserExists(ctx context.Context, email string) (bool, error)
}

type newUser interface {
	SaveNewUser(ctx context.Context, username string, email string, password []byte) (uid string, err error)
}

func New(
	log *zap.Logger,
	userAuth userAuth,
	nUser newUser) *Auth {
	return &Auth{
		log:      log,
		userAuth: userAuth,
		newUser:  nUser,
	}
}

func (auth *Auth) RegisterUser(ctx context.Context, username string, email string, password string) (userid string, err error) {
	const op = "auth/service.registernewuser"

	log := auth.log.With(
		zap.String(op, "attempting to register new user"),
		zap.String(username, "attempting to register user with this username"),
	)
	log.Info("start process of register new user")

	exist, err := auth.userAuth.UserExists(ctx, email)
	if err != nil {
		return "", fmt.Errorf("%s:%w", op, err)
	}
	if exist {
		return "", fmt.Errorf("user already exist")
	}

	hashedPass, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("%s:%w", op, err)
	}

	id, err := auth.newUser.SaveNewUser(ctx, username, email, hashedPass)
	if err != nil {
		return "", fmt.Errorf("%s:%w", op, err)
	}

	return id, err
}
