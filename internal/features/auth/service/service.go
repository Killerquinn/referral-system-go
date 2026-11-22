package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/killerquinn/referral-system-go/internal/domain/auth"
	"github.com/killerquinn/referral-system-go/internal/features/auth/dto"
	httpfeatures "github.com/killerquinn/referral-system-go/internal/infrastructure/pkg/http-features"
	"github.com/killerquinn/referral-system-go/internal/infrastructure/pkg/jwt"
	"github.com/killerquinn/referral-system-go/internal/infrastructure/pkg/refreshtoken"
	sharederrors "github.com/killerquinn/referral-system-go/internal/shared/shared-errors.go"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

type Auth struct {
	log                     *zap.Logger
	jwtsecret               []byte
	hmacsecret              []byte
	codehmacsecret          []byte
	tokenTTL                time.Duration
	refreshTTL              time.Duration
	codeTTL                 time.Duration
	userAuth                userAuth
	sRegister               sessionRegister
	newUser                 newUser
	passwordRecovery        passwordRecovery
	enqueuePasswordRecovery enqueuePasswordRecovery
	emailVerification       emailVerification
	emailVerifySender       emailVerifySender
}

type userAuth interface {
	UserExists(ctx context.Context, email string) error
}

type sessionRegister interface {
	CreateSession(ctx context.Context, userID uuid.UUID, hashedRefreshToken string, userAgent string, clientIP string, expiresAt time.Time) (refreshToken []byte, err error)
	DeleteSession(ctx context.Context, refreshHash string) error
	DeleteAllSessions(ctx context.Context, userID uuid.UUID) error
	RotateSession(ctx context.Context, givenRefresh string, usersRefresh string, ttl time.Time, metadata dto.SessionMetadata) (userID string, err error)
	DeleteExpiredSessions(ctx context.Context) (int64, error)
}

type newUser interface {
	User(ctx context.Context, email string) (auth.User, error) //get user
	SaveNewUser(ctx context.Context, username string, email string, password []byte) (uid string, err error)
}
type passwordRecovery interface {
	IssueCode(ctx context.Context, userID, purpose, hexHash string, expiresAt time.Time) error
	ResetPasswordWithCode(ctx context.Context, userID, purpose, codeHash string, newPassHash []byte, maxAttempts int) error
}
type enqueuePasswordRecovery interface {
	EnqueueSendResetCode(ctx context.Context, email, code string) error
}
type emailVerification interface {
	EmailStatus(ctx context.Context, userID string) (email string, verified bool, err error)
	VerifyEmailWithCode(ctx context.Context, userID, purpose, codeHash string, maxAttempts int) error
}
type emailVerifySender interface {
	EnqueueSendVerifyCode(ctx context.Context, email, code string) error
}

func New(
	log *zap.Logger,
	jwtsecret []byte,
	hmacsecret []byte,
	codehmacsecret []byte,
	tokenTTL time.Duration,
	refreshTTL time.Duration,
	codeTTL time.Duration,
	userAuth userAuth,
	sRegister sessionRegister,
	nUser newUser,
	passwordRecovery passwordRecovery,
	enqueuePasswordRecovery enqueuePasswordRecovery,
	emailVerification emailVerification,
	emailVerifySender emailVerifySender,
) *Auth {
	return &Auth{
		log:                     log,
		jwtsecret:               jwtsecret,
		hmacsecret:              hmacsecret,
		codehmacsecret:          codehmacsecret,
		tokenTTL:                tokenTTL,
		refreshTTL:              refreshTTL,
		codeTTL:                 codeTTL,
		userAuth:                userAuth,
		sRegister:               sRegister,
		newUser:                 nUser,
		passwordRecovery:        passwordRecovery,
		enqueuePasswordRecovery: enqueuePasswordRecovery,
		emailVerification:       emailVerification,
		emailVerifySender:       emailVerifySender,
	}
}

const (
	purposePassword = "password_recovery"
	purposeEmail    = "email_verify"
	// wrong guesses before the code is burned (and new codes are locked for a while, see processAttemptQuery)
	maxCodeAttempts = 5
)

func (auth *Auth) RegisterUser(ctx context.Context, username string, email string, password string) (userid string, err error) {
	const op = "auth/service.registernewuser"

	log := auth.log.With(
		zap.String(op, "attempting to register new user"),
		zap.String(username, "attempting to register user with this username"),
	)
	log.Info("start process of register new user")

	if err := auth.userAuth.UserExists(ctx, email); err != nil {
		if errors.Is(err, sharederrors.ErrUserAlreadyRegistered) {
			return "", sharederrors.ErrUserAlreadyRegistered
		}
		return "", fmt.Errorf("%s:%w", op, err)
	}

	hashedPass, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Error("error", zap.Error(err))
		return "", fmt.Errorf("%s:%w", op, err)
	}

	id, err := auth.newUser.SaveNewUser(ctx, username, email, hashedPass)
	if err != nil {
		if errors.Is(err, sharederrors.ErrNotUniqueUsername) {
			return "", sharederrors.ErrNotUniqueUsername
		}
		log.Error("error", zap.Error(err))
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

	err = auth.userAuth.UserExists(ctx, email)
	if err == nil {

		return "", "", sharederrors.ErrUserNotFound
	}

	user, err := auth.newUser.User(ctx, email)
	if err != nil {
		if errors.Is(err, sharederrors.ErrUserNotFound) { //To-Do: add errors to shared
			return "", "", sharederrors.ErrUserNotFound
		}
		log.Error("117 - error", zap.Error(err))
		return "", "", fmt.Errorf("%s:%w", op, err)
	}

	//To-Do: add comparing of user agent from DB to incoming request, to send warnings on users email that someone tries to log-in

	if err = bcrypt.CompareHashAndPassword(user.HashedPassword, []byte(password)); err != nil {
		return "", "", sharederrors.ErrInvalidCreds
	}

	// checked after the password so ban status isn't revealed to someone who only knows the email
	if user.IsBlocked {
		return "", "", sharederrors.ErrUserIsBanned
	}

	rawrefreshToken, rhash, err := refreshtoken.GenerateRefreshToken(auth.hmacsecret)
	if err != nil {
		log.Error("139 - error", zap.Error(err))
		return "", "", fmt.Errorf("%s:%w", op, err)
	}

	accessToken, err := jwt.GenerateAccessToken(user.ID.String(), string(auth.jwtsecret), auth.tokenTTL)
	if err != nil {
		log.Error("145 - error", zap.Error(err))
		return "", "", fmt.Errorf("%s:%w", op, err)
	}

	_, err = auth.sRegister.CreateSession(ctx, user.ID, rhash, userAgent, userIP, time.Now().Add(auth.refreshTTL))
	if err != nil {
		log.Error("151 - error", zap.Error(err))
		return "", "", fmt.Errorf("%s:%w", op, err)
	}

	return accessToken, rawrefreshToken, nil
}

func (auth *Auth) Logout(ctx context.Context, rawRefresh string) error {
	const op = "auth/service.Logout"

	log := auth.log.With(
		zap.String(op, "trying to logout user"),
	)

	if err := auth.sRegister.DeleteSession(ctx, refreshtoken.HashToken(rawRefresh, auth.hmacsecret)); err != nil {
		log.Error("unable to delete session", zap.Error(err))
		return fmt.Errorf("%s:%w", op, err)
	}

	return nil
}

func (auth *Auth) LogoutAll(ctx context.Context, userID string) error {
	const op = "auth/service.LogoutAll"

	log := auth.log.With(
		zap.String(op, "trying to logout user from all devices"),
	)

	uuID, err := uuid.Parse(userID)
	if err != nil {
		return fmt.Errorf("%s:%w", op, err)
	}

	if err = auth.sRegister.DeleteAllSessions(ctx, uuID); err != nil {
		log.Error("unable to delete user's sessions", zap.Error(err))
		return fmt.Errorf("%s:%w", op, err)
	}

	log.Info("all user's sessions successfully deleted")
	return nil
}

func (auth *Auth) Refresh(ctx context.Context, rawtoken string, metadata dto.SessionMetadata) (tokens *dto.Tokens, err error) {
	const op = "auth/service.Refresh"

	log := auth.log.With(
		zap.String(op, "op"),
	)

	newRaw, newHash, err := refreshtoken.GenerateRefreshToken(auth.hmacsecret)
	if err != nil {
		log.Error("err", zap.Error(err))
		return nil, fmt.Errorf("%s:%w", op, err)
	}

	userID, err := auth.sRegister.RotateSession(ctx, refreshtoken.HashToken(rawtoken, auth.hmacsecret), newHash,
		time.Now().Add(auth.refreshTTL), metadata)
	if err != nil {
		if errors.Is(err, sharederrors.ErrInvalidRefresh) {
			return nil, sharederrors.ErrInvalidRefresh
		}
		log.Error("err", zap.Error(err))
		return nil, fmt.Errorf("%s:%w", op, err)
	}

	access, err := jwt.GenerateAccessToken(userID, string(auth.jwtsecret), auth.tokenTTL) // same func that login use
	if err != nil {
		log.Error("err", zap.Error(err))
		return nil, fmt.Errorf("%s:%w", op, err)
	}

	return &dto.Tokens{Access: access, Refresh: newRaw}, nil
}

func (auth *Auth) CleanupExpiredSessions(ctx context.Context) (int, error) {
	const op = "auth/service.CleanupExpiredSessions"

	log := auth.log.With(
		zap.String("op", op),
	)

	deleted, err := auth.sRegister.DeleteExpiredSessions(ctx)
	if err != nil {
		log.Error("error while tried to cleanup expired sessions", zap.Error(err))
		return 0, fmt.Errorf("%s:%w", op, err)
	}

	return int(deleted), nil
}

func (auth *Auth) ForgotPass(ctx context.Context, email string) (err error) {
	const op = "auth/service.ForgotPass"

	log := auth.log.With(
		zap.String("op", op),
	)

	rawCode, err := httpfeatures.GenerateCode()
	if err != nil {
		log.Error("err occured while feature generated code", zap.Error(err))
		return fmt.Errorf("%s:%w", op, err)
	}

	user, err := auth.newUser.User(ctx, email)
	if err != nil {
		if errors.Is(err, sharederrors.ErrUserNotFound) {
			return nil
		}
		log.Error("err occured while feature tried to find user", zap.Error(err))
		return fmt.Errorf("%s:%w", op, err)
	}

	if user.IsBlocked {
		return nil
	}

	userID := user.ID.String()

	hashedCode := httpfeatures.HashCode(rawCode, userID, purposePassword, auth.codehmacsecret)

	if err := auth.passwordRecovery.IssueCode(ctx, userID, purposePassword, hashedCode, time.Now().Add(auth.codeTTL)); err != nil {
		if errors.Is(err, sharederrors.ErrCodeCooldown) {
			return nil
		}
		log.Error("error occured while feature tried to set code", zap.Error(err))
		return fmt.Errorf("%s:%w", op, err)
	}

	//TODO: enqueue send code

	if err := auth.enqueuePasswordRecovery.EnqueueSendResetCode(ctx, user.Email, rawCode); err != nil {
		log.Error("error while tried to start EnqueueSendResetCode", zap.Error(err))
		return nil
	}

	return nil
}

// ResetPassword checks the emailed code and sets a new password. Every "no" (unknown email, banned user,
// no / expired / locked / wrong code) is the same ErrInvalidCode, so the response doesn't reveal which one it was.
// On success the code is deleted and all sessions are dropped: the user logs in again with the new password.
func (auth *Auth) ResetPassword(ctx context.Context, email, code, newPassword string) error {
	const op = "auth/service.ResetPassword"

	log := auth.log.With(
		zap.String("op", op),
	)

	user, err := auth.newUser.User(ctx, email)
	if err != nil {
		if errors.Is(err, sharederrors.ErrUserNotFound) {
			return sharederrors.ErrInvalidCode
		}
		log.Error("err occured while feature tried to find user", zap.Error(err))
		return fmt.Errorf("%s:%w", op, err)
	}

	if user.IsBlocked {
		return sharederrors.ErrInvalidCode
	}

	userID := user.ID.String()

	// bcrypt is slow on purpose: hash before the repo opens its transaction, so the row lock is held briefly
	newPassHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		log.Error("err occured while hashing new password", zap.Error(err))
		return fmt.Errorf("%s:%w", op, err)
	}

	codeHash := httpfeatures.HashCode(code, userID, purposePassword, auth.codehmacsecret)

	if err := auth.passwordRecovery.ResetPasswordWithCode(ctx, userID, purposePassword, codeHash, newPassHash, maxCodeAttempts); err != nil {
		if errors.Is(err, sharederrors.ErrInvalidCode) {
			return sharederrors.ErrInvalidCode
		}
		log.Error("err occured while resetting password", zap.Error(err))
		return fmt.Errorf("%s:%w", op, err)
	}

	log.Info("password reset", zap.String("user_id", userID))
	return nil
}

// SendEmailVerification mails a code to the logged-in user's address. Unlike ForgotPass the errors are honest
// (already verified / cooldown): the caller is authenticated, there's no email to enumerate.
func (auth *Auth) SendEmailVerification(ctx context.Context, userID string) error {
	const op = "auth/service.SendEmailVerification"

	log := auth.log.With(
		zap.String("op", op),
		zap.String("user_id", userID),
	)

	email, verified, err := auth.emailVerification.EmailStatus(ctx, userID)
	if err != nil {
		if errors.Is(err, sharederrors.ErrUserNotFound) {
			return sharederrors.ErrUserNotFound
		}
		log.Error("err occured while getting email status", zap.Error(err))
		return fmt.Errorf("%s:%w", op, err)
	}
	if verified {
		return sharederrors.ErrEmailAlreadyVerified
	}

	rawCode, err := httpfeatures.GenerateCode()
	if err != nil {
		log.Error("err occured while generating code", zap.Error(err))
		return fmt.Errorf("%s:%w", op, err)
	}

	codeHash := httpfeatures.HashCode(rawCode, userID, purposeEmail, auth.codehmacsecret)

	if err := auth.passwordRecovery.IssueCode(ctx, userID, purposeEmail, codeHash, time.Now().Add(auth.codeTTL)); err != nil {
		if errors.Is(err, sharederrors.ErrCodeCooldown) {
			return sharederrors.ErrCodeCooldown
		}
		log.Error("err occured while issuing code", zap.Error(err))
		return fmt.Errorf("%s:%w", op, err)
	}

	if err := auth.emailVerifySender.EnqueueSendVerifyCode(ctx, email, rawCode); err != nil {
		log.Error("err occured while enqueueing verify email", zap.Error(err))
		return fmt.Errorf("%s:%w", op, err)
	}

	return nil
}

// VerifyEmail checks the code and marks the email as verified.
func (auth *Auth) VerifyEmail(ctx context.Context, userID, code string) error {
	const op = "auth/service.VerifyEmail"

	log := auth.log.With(
		zap.String("op", op),
		zap.String("user_id", userID),
	)

	_, verified, err := auth.emailVerification.EmailStatus(ctx, userID)
	if err != nil {
		if errors.Is(err, sharederrors.ErrUserNotFound) {
			return sharederrors.ErrUserNotFound
		}
		log.Error("err occured while getting email status", zap.Error(err))
		return fmt.Errorf("%s:%w", op, err)
	}
	if verified {
		return sharederrors.ErrEmailAlreadyVerified
	}

	codeHash := httpfeatures.HashCode(code, userID, purposeEmail, auth.codehmacsecret)

	if err := auth.emailVerification.VerifyEmailWithCode(ctx, userID, purposeEmail, codeHash, maxCodeAttempts); err != nil {
		if errors.Is(err, sharederrors.ErrInvalidCode) {
			return sharederrors.ErrInvalidCode
		}
		log.Error("err occured while verifying email", zap.Error(err))
		return fmt.Errorf("%s:%w", op, err)
	}

	log.Info("email verified")
	return nil
}
