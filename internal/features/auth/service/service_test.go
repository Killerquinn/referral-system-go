package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"

	"github.com/killerquinn/referral-system-go/internal/domain/auth"
	"github.com/killerquinn/referral-system-go/internal/features/auth/service"
)

// --- Mocks ---

type mockUserAuth struct {
	exists bool
	err    error
	banned bool
	blerr  error
}

func (m mockUserAuth) UserExists(ctx context.Context, email string) (bool, error) {
	return m.exists, m.err
}

func (m mockUserAuth) UserIsBlocked(ctx context.Context, userID string) (bool, error) {
	return m.banned, m.blerr
}

type mockSessionRegister struct {
	token  []byte
	err    error
	delerr error
}

func (m mockSessionRegister) CreateSession(ctx context.Context, userID uuid.UUID, hashedRefreshToken string, userAgent string, clientIP string, expiresAt time.Time) ([]byte, error) {
	return m.token, m.err
}
func (m mockSessionRegister) DeleteCurrentSession(ctx context.Context, userID uuid.UUID) error {
	return m.delerr
}

type mockNewUser struct {
	user        *auth.User
	userErr     error
	savedUserID string
	saveErr     error
}

func (m mockNewUser) User(ctx context.Context, email string) (*auth.User, error) {
	return m.user, m.userErr
}

func (m mockNewUser) SaveNewUser(ctx context.Context, username string, email string, password []byte) (string, error) {
	return m.savedUserID, m.saveErr
}

// --- Tests ---

func TestRegisterUser(t *testing.T) {
	logger := zap.NewNop()
	secret := []byte("secret")
	ttl := 30 * 24 * time.Hour

	t.Run("success", func(t *testing.T) {
		authSrv := service.New(
			logger,
			secret,
			ttl,
			mockUserAuth{exists: false},
			mockSessionRegister{},
			mockNewUser{savedUserID: "uuid-123"},
		)

		id, err := authSrv.RegisterUser(context.Background(), "artem", "artem@example.com", "pass123")

		require.NoError(t, err)
		assert.Equal(t, "uuid-123", id)
	})

	t.Run("user_already_exists", func(t *testing.T) {
		authSrv := service.New(
			logger,
			secret,
			ttl,
			mockUserAuth{exists: true},
			mockSessionRegister{},
			mockNewUser{},
		)

		_, err := authSrv.RegisterUser(context.Background(), "artem", "artem@example.com", "pass123")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "user already exist")
	})

	t.Run("db_error", func(t *testing.T) {
		authSrv := service.New(
			logger,
			secret,
			ttl,
			mockUserAuth{err: errors.New("db error")},
			mockSessionRegister{},
			mockNewUser{},
		)

		_, err := authSrv.RegisterUser(context.Background(), "artem", "artem@example.com", "pass123")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "db error")
	})
}

func TestLogin(t *testing.T) {
	logger := zap.NewNop()
	secret := []byte("secret")
	ttl := 30 * 24 * time.Hour

	passHash, _ := bcrypt.GenerateFromPassword([]byte("correct_password"), bcrypt.DefaultCost)
	validUser := &auth.User{
		ID:             uuid.New(),
		HashedPassword: passHash,
	}

	t.Run("user_not_found", func(t *testing.T) {
		authSrv := service.New(
			logger,
			secret,
			ttl,
			mockUserAuth{exists: false},
			mockSessionRegister{},
			mockNewUser{},
		)

		_, _, err := authSrv.Login(context.Background(), "notfound@example.com", "pass", "agent", "127.0.0.1")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "user doesnt even exist!")
	})

	t.Run("invalid_credentials", func(t *testing.T) {
		authSrv := service.New(
			logger,
			secret,
			ttl,
			mockUserAuth{exists: true},
			mockSessionRegister{},
			mockNewUser{user: validUser},
		)

		_, _, err := authSrv.Login(context.Background(), "artem@example.com", "wrong_password", "agent", "127.0.0.1")

		require.Error(t, err)
		assert.EqualError(t, err, "invalid credentials")
	})

	t.Run("session_creation_failed", func(t *testing.T) {
		authSrv := service.New(
			logger,
			secret,
			ttl,
			mockUserAuth{exists: true},
			mockSessionRegister{err: errors.New("db write failed")},
			mockNewUser{user: validUser},
		)

		_, _, err := authSrv.Login(context.Background(), "artem@example.com", "correct_password", "agent", "127.0.0.1")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "db write failed")
	})
}
