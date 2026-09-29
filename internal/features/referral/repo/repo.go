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

func (r *Repository) GetReferralsByRefferersUsername(ctx context.Context, username string, cursor *time.Time, lastID *string, limit int) (entities []dto.ReferralItem, err error) {
	const op = "referral-system-go/internal/features/referral/repo GetReferralsByRefferersUsername"

	conn, err := r.GetConn()
	if err != nil {

		return nil, fmt.Errorf("%s:%w", op, err)
	}

	defer conn.Release()

	var banned bool

	if err := conn.QueryRow(ctx, CheckIfUserIsBanned, username).Scan(&banned); err != nil {

		return nil, sharederrors.ErrUserIsBanned
	}

	rows, err := conn.Query(ctx, GetReferralsByUsername, username, cursor, lastID, limit)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {

			return nil, sharederrors.ErrUserNotFound
		}

		return nil, fmt.Errorf("%s:%w", op, err)
	}

	resp, err := pgx.CollectRows(rows, pgx.RowToStructByName[dto.ReferralItem])
	if err != nil {
		return nil, fmt.Errorf("%s:%w", op, err)
	}
	if len(resp) == 0 {
		return nil, sharederrors.ErrUserDoesntHaveReferralsYet
	}

	return resp, nil
}

func (r *Repository) GetReferralsByOwnUserID(ctx context.Context, ownUserID string, cursor *time.Time, lastID *string, limit int) (entities []dto.ReferralItem, err error) {
	const op = "referral-system-go/internal/features/referral/repo GetReferralsByOwnUserID"

	conn, err := r.GetConn()
	if err != nil {

		return nil, fmt.Errorf("%s:%w", op, err)
	}

	defer conn.Release()

	rows, err := conn.Query(ctx, GetReferralsByOwnUserID, ownUserID, cursor, lastID, limit)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {

			return nil, sharederrors.ErrUserNotFound
		}

		return nil, fmt.Errorf("%s:%w", op, err)
	}

	resp, err := pgx.CollectRows(rows, pgx.RowToStructByName[dto.ReferralItem])
	if err != nil {
		return nil, fmt.Errorf("%s:%w", op, err)
	}
	if len(resp) == 0 {
		return nil, sharederrors.ErrUserDoesntHaveReferralsYet
	}

	return resp, nil
}

func (r *Repository) GetActualQuantityOfReferrers(ctx context.Context, userid string, startFrom *time.Time, endUntil *time.Time) (int, error) {
	const op = "referral-system-go/internal/features/referral/repo GetActualQuantityOfReferrers"

	conn, err := r.GetConn()
	if err != nil {

		return 0, fmt.Errorf("%s:%w", op, err)
	}

	defer conn.Release()

	var count int

	if err := conn.QueryRow(ctx, GetQuantityOfReferrers, userid, startFrom, endUntil).Scan(&count); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {

			return 0, sharederrors.ErrInsufficientQuantityOfReferrals
		}
		return 0, fmt.Errorf("%s:%w", op, err)
	}

	return count, nil
}

func (r *Repository) GetWinners(ctx context.Context, offsets []int, sponsorID string, startFrom *time.Time, endUntil *time.Time) ([]dto.WinnersDTO, error) {
	const op = "referral-system-go/internal/features/referral/repo GetWinners"

	conn, err := r.GetConn()
	if err != nil {

		return nil, fmt.Errorf("%s:%w", op, err)
	}

	defer conn.Release()

	rows, err := conn.Query(ctx, GetRandomUsersByOffset, offsets, sponsorID, startFrom, endUntil)
	if err != nil {
		return nil, fmt.Errorf("%s:%w", op, err)
	}

	defer rows.Close()

	winners, err := pgx.CollectRows(rows, pgx.RowToStructByNameLax[dto.WinnersDTO])
	if err != nil {
		return nil, fmt.Errorf("%s:%w", op, err)
	}
	return winners, nil
}
