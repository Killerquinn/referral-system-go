package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

type MockUserAuth struct {
	mock.Mock
}

func (m *MockUserAuth) UserExists(ctx context.Context, email string) (bool, error) {
	args := m.Called(ctx, email)
	return args.Bool(0), args.Error(1)
}

type MockNewUser struct {
	mock.Mock
}

func (m *MockNewUser) SaveNewUser(ctx context.Context, username string, email string, password []byte) (string, error) {
	args := m.Called(ctx, username, email, password)
	return args.String(0), args.Error(1)
}

func TestRegisterUser(t *testing.T) {
	var (
		ctx      = context.Background()
		username = "john_doe"
		email    = "john@example.com"
		password = "secretpassword"
	)

	tests := []struct {
		name        string
		username    string
		email       string
		password    string
		setupMocks  func(mAuth *MockUserAuth, mNewUser *MockNewUser)
		expectedID  string
		expectedErr string
		expectErr   bool
	}{
		{
			name:     "Success registration",
			username: username,
			email:    email,
			password: password,
			setupMocks: func(mAuth *MockUserAuth, mNewUser *MockNewUser) {
				mAuth.On("UserExists", ctx, email).Return(false, nil)

				mNewUser.On("SaveNewUser", ctx, username, email, mock.MatchedBy(func(hashed []byte) bool {
					err := bcrypt.CompareHashAndPassword(hashed, []byte(password))
					return err == nil
				})).Return("usr_12345", nil)
			},
			expectedID: "usr_12345",
			expectErr:  false,
		},
		{
			name:     "User already exists",
			username: username,
			email:    email,
			password: password,
			setupMocks: func(mAuth *MockUserAuth, mNewUser *MockNewUser) {
				mAuth.On("UserExists", ctx, email).Return(true, nil)
			},
			expectedID:  "",
			expectedErr: "user already exist",
			expectErr:   true,
		},
		{
			name:     "Error on UserExists DB call",
			username: username,
			email:    email,
			password: password,
			setupMocks: func(mAuth *MockUserAuth, mNewUser *MockNewUser) {
				mAuth.On("UserExists", ctx, email).Return(false, errors.New("db connection timeout"))
			},
			expectedID:  "",
			expectedErr: "auth/service.registernewuser:db connection timeout",
			expectErr:   true,
		},
		{
			name:     "Error on SaveNewUser DB call",
			username: username,
			email:    email,
			password: password,
			setupMocks: func(mAuth *MockUserAuth, mNewUser *MockNewUser) {
				mAuth.On("UserExists", ctx, email).Return(false, nil)
				mNewUser.On("SaveNewUser", ctx, username, email, mock.Anything).
					Return("", errors.New("unique constraint violation"))
			},
			expectedID:  "",
			expectedErr: "auth/service.registernewuser:unique constraint violation",
			expectErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mAuth := new(MockUserAuth)
			mNewUser := new(MockNewUser)

			tt.setupMocks(mAuth, mNewUser)

			authService := New(zap.NewNop(), mAuth, mNewUser)

			id, err := authService.RegisterUser(ctx, tt.username, tt.email, tt.password)

			if tt.expectErr {
				assert.Error(t, err)
				assert.EqualError(t, err, tt.expectedErr)
				assert.Empty(t, id)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedID, id)
			}

			mAuth.AssertExpectations(t)
			mNewUser.AssertExpectations(t)
		})
	}
}
