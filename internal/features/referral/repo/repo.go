package repo

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/killerquinn/referral-system-go/internal/features/referral/dto"
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

func (r *Repository) GetConn() (*pgxpool.Conn, error) {
	const op = "user/repo.GetConn"

	conn, err := r.db.Acquire(context.Background())
	if err != nil {
		log.Panicf("%s - %s", op, err)
	}

	return conn, err
}

func (r *Repository) CheckReferralByUsername(ctx context.Context, username string) (referrersusername string, referredSince time.Time, err error) {
	const op = "/referral-system-go/internal/features/referral/repo"

	conn, err := r.GetConn()
	if err != nil {
		return "", time.Time{}, fmt.Errorf("%s:%w", op, err)
	}

	defer conn.Release()

	var referrerUserID string
	var lastTimeReferred time.Time

	if err := conn.QueryRow(ctx, FindReferrerByReferralUsername, username).Scan(&referrerUserID, &lastTimeReferred); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {

			return "", time.Time{}, sharederrors.ErrUserNotFound
		}

		return "", time.Time{}, fmt.Errorf("%s:%w", op, err)
	}

	var referrerUsername string

	if err := conn.QueryRow(ctx, GetReferrerByUserID, referrerUserID).Scan(&referrerUsername); err != nil {

		return "", time.Time{}, fmt.Errorf("%s:%w", op, err)
	}

	return referrerUsername, lastTimeReferred, nil
}

func (r *Repository) GetReferralsByRefferersUsername(ctx context.Context, username string, cursor string, limit int) (entities []dto.ReferralItem, err error) {
	panic("implement me!")
}

func (r *Repository) GetReferralsByOwnUserID(ctx context.Context, username string, cursor string, limit int) (entities []dto.ReferralItem, err error) {
	panic("implement me!")
}
