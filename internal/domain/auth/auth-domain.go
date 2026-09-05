package auth

import (
	"time"

	"github.com/google/uuid"
)

type Session struct {
	ID               uuid.UUID
	UserID           uuid.UUID
	RefreshTokenHash string
	UserAgent        string
	ClientIP         string
	IsBlocked        bool
	ExpiresAt        time.Time
	CreatedAt        time.Time
}

type User struct {
	ID              uuid.UUID `db:"id"`
	Username        string    `db:"username"`
	Email           string    `db:"email"`
	HashedPassword  []byte    `db:"hashed_password"`
	OwnReferral     string    `db:"own_referral"`
	ReferredBy      string    `db:"referred_by"`
	LastTimeRefused time.Time `db:"last_time_refused"`
	CreatedAt       time.Time `db:"created_at"`
}

func (s *Session) IsExpired() bool {
	return time.Now().After(s.ExpiresAt)
}

func (s *Session) TimeToLive() time.Duration {
	if s.IsExpired() {
		return 0
	}
	return time.Until(s.ExpiresAt)
}
func (s *Session) SetDefaultTTL(duration time.Duration) {
	now := time.Now()
	s.CreatedAt = now
	s.ExpiresAt = now.Add(duration)
}
