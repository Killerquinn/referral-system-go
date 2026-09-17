package repo

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/killerquinn/referral-system-go/internal/domain/auth"
	sharederrors "github.com/killerquinn/referral-system-go/internal/shared/shared-errors.go"
)

type Repository struct {
	db *pgxpool.Pool
}

func New(dsn string) *Repository {
	const op = "user/repo.New"

	db, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		log.Panicf("%s - %s", op, err)
	} //add error handling without panic

	if db == nil {
		log.Panicf("%s - %s", op, "db is nil")
	}

	if err := db.Ping(context.Background()); err != nil {
		log.Panicf("%s - %s", op, err)
	}

	return &Repository{
		db: db,
	}
}

func (db *Repository) GetConn() (*pgxpool.Conn, error) {
	const op = "user/repo.GetConn"

	conn, err := db.db.Acquire(context.Background())
	if err != nil {
		log.Panicf("%s - %s", op, err)
	}

	return conn, err
}

func (r *Repository) PullOldPassword(ctx context.Context, userID uuid.UUID) (oldPass []byte, err error) {
	const op = "/internal/features/user/repo.PullOldPassword"

	conn, err := r.GetConn()
	if err != nil {
		return nil, fmt.Errorf("%s:%w", op, err)
	}

	defer conn.Release()

	var oldpassword string

	if err := conn.QueryRow(ctx, SelectHashedPassword, userID).Scan(&oldpassword); err != nil {
		return nil, fmt.Errorf("%s:%w", op, err)
	}

	return []byte(oldpassword), nil

}

func (r *Repository) ChangePassword(ctx context.Context, userID uuid.UUID, newPassword []byte) (err error) {
	const op = "/internal/features/user/repo.ChangePassword"

	conn, err := r.GetConn()
	if err != nil {
		return fmt.Errorf("%s:%w", op, err)
	}

	defer conn.Release()

	result, err := conn.Exec(ctx, UpdateCurrentPassword, string(newPassword), userID)
	if err != nil {
		return fmt.Errorf("%s:%w", op, err)
	}

	affectedRows := result.RowsAffected()
	if affectedRows == 0 {
		return sharederrors.ErrUnnableToChangePassword
	}

	return nil
}

func (r *Repository) IfAbleToChangeReferrer(ctx context.Context, userID uuid.UUID) (availableafter time.Time, err error) {
	const op = "/internal/features/user/repo.IfAbleToChangeReferrer"

	conn, err := r.GetConn()
	if err != nil {
		return time.Time{}, fmt.Errorf("%s:%w", op, err)
	}

	defer conn.Release()

	var ableafter time.Time

	if err := conn.QueryRow(ctx, CheckIfAbleToChangeReferrer, userID).Scan(&ableafter); err != nil {

		return time.Time{}, fmt.Errorf("%s:%w", op, err)
	}

	return ableafter, nil
}

func (r *Repository) ChangeCurrentReferrer(ctx context.Context, userID uuid.UUID, refcode string, newTimestamp time.Time) (err error) {
	const op = "/internal/features/user/repo.ChangeCurrentReferrer"

	conn, err := r.GetConn()
	if err != nil {
		return fmt.Errorf("%s:%w", op, err)
	}

	defer conn.Release()

	//checking if user doesnt tries to be referrer to himself
	var referredUser auth.User
	var referrer auth.User
	if err = conn.QueryRow(ctx, CheckOnSelfReferralAndFindReferrerID, refcode, userID).Scan(&referredUser.ID, &referredUser.LastTimeRefUsed, &referredUser.ReferredBy, &referrer.ReferredBy); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {

			return sharederrors.ErrReferrerOrReferralCodeDoesntExist
		}

		return fmt.Errorf("%s:%w", op, err)
	}

	//todo: add sync with referrals SQL table, add check on circular referral, add registration of new referral

	panic("implement me!")
}

func (r *Repository) IfReferrerExist(ctx context.Context, refcode string) (err error) {
	const op = "/internal/features/user/repo.IfReferrerExist"

	conn, err := r.GetConn()
	if err != nil {
		return fmt.Errorf("%s:%w", op, err)
	}

	defer conn.Release()

	panic("implement me!")
}
