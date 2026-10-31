package repopackage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/killerquinn/referral-system-go/internal/domain/auth"
	"github.com/killerquinn/referral-system-go/internal/features/auth/dto"
	sharederrors "github.com/killerquinn/referral-system-go/internal/shared/shared-errors.go"
)

type Repository struct {
	db *pgxpool.Pool
}

func New(db *pgxpool.Pool) *Repository {
	return &Repository{
		db: db,
	}
}

func (r *Repository) User(ctx context.Context, email string) (auth.User, error) {
	const op = "user/repo.User"

	rows, err := r.db.Query(ctx, getUserByEmail, email)
	if err != nil {
		return auth.User{}, fmt.Errorf("%s:%w", op, err)
	}
	defer rows.Close()

	user, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[auth.User])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return auth.User{}, sharederrors.ErrUserNotFound
		}
		return auth.User{}, fmt.Errorf("%s:%w", op, err)
	}

	return user, nil
}

func (r *Repository) UserExists(ctx context.Context, email string) error { //To-Do: make repository shared to avoid connection collisions and else(I'm building monolith, not microservices)
	const op = "user/repo.UserExist"

	var isUserExist bool

	err := r.db.QueryRow(ctx, selectIfUserExist, email).Scan(&isUserExist)
	if err != nil {
		return fmt.Errorf("%s:%w", op, err)
	}

	if isUserExist {
		return sharederrors.ErrUserAlreadyRegistered
	}

	return nil
}

func (r *Repository) SaveNewUser(ctx context.Context, username string, email string, password []byte) (uid string, err error) {
	const op = "user/repo.SaveNewUser"

	var id string

	err = r.db.QueryRow(ctx, createUserQuery, username, email, password).Scan(&id)
	if err != nil {
		if pgerr, ok := err.(*pgconn.PgError); ok && pgerr.Code == "23505" {
			return "", sharederrors.ErrUserAlreadyRegistered
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return "", sharederrors.ErrNotUniqueUsername
		}
		return "", fmt.Errorf("%s:%w", op, err)
	}

	return id, nil

}

func (r *Repository) CreateSession(ctx context.Context, userID uuid.UUID, hashedRefreshToken string, userAgent string, clientIP string, expiresAt time.Time) (refreshToken []byte, err error) {
	const op = "user/repo.CreateSession"

	var rToken string

	uid := userID.String()

	err = r.db.QueryRow(ctx, postSession, uid, hashedRefreshToken, userAgent, clientIP, expiresAt).Scan(&rToken)
	if err != nil {
		return nil, fmt.Errorf("%s:%w", op, err)
	}

	return []byte(rToken), nil
}

// DeleteSession and DeleteAllSessions are idempotent: deleting an already
// missing session is not an error, the client ends up logged out either way
func (r *Repository) DeleteSession(ctx context.Context, refreshHash string) error {
	const op = "user/repo.DeleteSession"

	if _, err := r.db.Exec(ctx, dropSessionByRefreshHash, refreshHash); err != nil {
		return fmt.Errorf("%s:%w", op, err)
	}
	return nil
}

func (r *Repository) DeleteAllSessions(ctx context.Context, userUUID uuid.UUID) error {
	const op = "user/repo.DeleteAllSessions"

	if _, err := r.db.Exec(ctx, dropAllUserSessions, userUUID); err != nil {
		return fmt.Errorf("%s:%w", op, err)
	}
	return nil
}

func (r *Repository) RotateSession(ctx context.Context, givenRefresh string, newRefresh string, ttl time.Time, metadata dto.SessionMetadata) (userID string, err error) {
	const op = "user/repo.RotateSession"

	if err := r.db.QueryRow(ctx, rotateSession, givenRefresh, newRefresh, ttl, metadata.UserAgent, metadata.ClientIP).Scan(&userID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", sharederrors.ErrInvalidRefresh
		}

		return "", fmt.Errorf("%s:%w", op, err)
	}

	return userID, err
}
