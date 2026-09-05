package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/killerquinn/referral-system-go/internal/domain/auth"
	"github.com/killerquinn/referral-system-go/internal/infrastructure/pkg/jwt"
	"github.com/killerquinn/referral-system-go/internal/infrastructure/pkg/refreshtoken"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

type Auth struct {
	log       *zap.Logger
	jwtsecret []byte
	tokenTTL  time.Duration
	userAuth  userAuth
	sRegister sessionRegister
	newUser   newUser
}

type userAuth interface {
	UserExists(ctx context.Context, email string) (bool, error)
	//CredsMatch(ctx context.Context, email string, password []byte) (matched bool, err error) //To-Do: resolve that
}

type sessionRegister interface {
	CreateSession(ctx context.Context, userID uuid.UUID, hashedRefreshToken string, userAgent string, clientIP string, expiresAt time.Time) (refreshToken []byte, err error)
}

type newUser interface {
	User(ctx context.Context, email string) (*auth.User, error) //get user
	SaveNewUser(ctx context.Context, username string, email string, password []byte) (uid string, err error)
}

func New(
	log *zap.Logger,
	jwtsecret []byte,
	tokenTTL time.Duration,
	userAuth userAuth,
	sRegister sessionRegister,
	nUser newUser) *Auth {
	return &Auth{
		log:       log,
		jwtsecret: jwtsecret,
		tokenTTL:  tokenTTL,
		userAuth:  userAuth,
		sRegister: sRegister,
		newUser:   nUser,
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

func (auth *Auth) Login(ctx context.Context, email string, password string, userAgent string, userIP string) (actoken string, rtoken string, err error) {
	const op = "auth/service.login"

	log := auth.log.With(
		zap.String(op, "attempting to create new session for (already existing?) user"),
	)
	log.Info("start proccess of logging ig new user")

	exist, err := auth.userAuth.UserExists(ctx, email)
	if err != nil {
		return "", "", fmt.Errorf("%s:%w", op, err)
	}
	if !exist {
		return "", "", fmt.Errorf("%s:%s", op, "user doesnt even exist!")
	}

	user, err := auth.newUser.User(ctx, email)
	if err != nil {
		if errors.Is(err, fmt.Errorf("errUserNotFound")) { //To-Do: add errors to shared
			return "", "", fmt.Errorf("%s:%w", op, fmt.Errorf("errUserNotFound"))
		}
		return "", "", fmt.Errorf("%s:%w", op, err)
	}
	//To-Do: add comparing of user agent from DB to incoming request, to send warnings on users email that someone tries to log-in

	if err = bcrypt.CompareHashAndPassword(user.HashedPassword, []byte(password)); err != nil {
		return "", "", fmt.Errorf("invalid credentials")
	}

	refreshToken, err := refreshtoken.GenerateRefreshToken()
	if err != nil {
		return "", "", fmt.Errorf("%s:%w", op, err)
	}
	accessToken, err := jwt.GenerateAccessToken(user.ID.String(), string(auth.jwtsecret))
	if err != nil {
		return "", "", fmt.Errorf("%s:%w", op, err)
	}
	rToken, err := auth.sRegister.CreateSession(ctx, user.ID, string(refreshToken), userAgent, userIP, time.Now().Add(auth.tokenTTL))
	if err != nil {
		return "", "", fmt.Errorf("%s:%w", op, err)
	}
	preparedRToken, err := refreshtoken.UnhashToken(rToken)
	if err != nil {
		return "", "", fmt.Errorf("%s:%w", op, err)
	}
	return accessToken, preparedRToken, nil
}
