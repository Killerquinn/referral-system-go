package repo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/killerquinn/referral-system-go/internal/features/referral/dto"
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

func (r *Repository) CheckReferralByUsername(ctx context.Context, username string) (referrersusername string, referredSince time.Time, err error) {
	const op = "/referral-system-go/internal/features/referral/repo"

	var referrerUserID string
	var lastTimeReferred time.Time

	if err := r.db.QueryRow(ctx, FindReferrerByReferralUsername, username).Scan(&referrerUserID, &lastTimeReferred); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {

			return "", time.Time{}, sharederrors.ErrUserNotFound
		}

		return "", time.Time{}, fmt.Errorf("%s:%w", op, err)
	}

	var referrerUsername string

	if err := r.db.QueryRow(ctx, GetReferrerByUserID, referrerUserID).Scan(&referrerUsername); err != nil {

		return "", time.Time{}, fmt.Errorf("%s:%w", op, err)
	}

	return referrerUsername, lastTimeReferred, nil
}

func (r *Repository) GetReferralsByRefferersUsername(ctx context.Context, username string, cursor *time.Time, lastID *string, limit int) (entities []dto.ReferralItem, err error) {
	const op = "referral-system-go/internal/features/referral/repo GetReferralsByRefferersUsername"

	var banned bool

	if err := r.db.QueryRow(ctx, CheckIfUserIsBanned, username).Scan(&banned); err != nil {

		return nil, sharederrors.ErrUserIsBanned
	}

	rows, err := r.db.Query(ctx, GetReferralsByUsername, username, cursor, lastID, limit)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {

			return nil, sharederrors.ErrUserNotFound
		}

		return nil, fmt.Errorf("%s:%w", op, err)
	}

	resp, err := pgx.CollectRows(rows, pgx.RowToStructByNameLax[dto.ReferralItem])
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

	rows, err := r.db.Query(ctx, GetReferralsByOwnUserID, ownUserID, cursor, lastID, limit)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {

			return nil, sharederrors.ErrUserNotFound
		}

		return nil, fmt.Errorf("%s:%w", op, err)
	}

	resp, err := pgx.CollectRows(rows, pgx.RowToStructByNameLax[dto.ReferralItem])
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

	var count int

	if err := r.db.QueryRow(ctx, GetQuantityOfReferrers, userid, startFrom, endUntil).Scan(&count); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {

			return 0, sharederrors.ErrInsufficientQuantityOfReferrals
		}
		return 0, fmt.Errorf("%s:%w", op, err)
	}

	return count, nil
}

func (r *Repository) GetWinners(ctx context.Context, offsets []int, sponsorID string, startFrom *time.Time, endUntil *time.Time) ([]dto.WinnersDTO, error) {
	const op = "referral-system-go/internal/features/referral/repo GetWinners"

	rows, err := r.db.Query(ctx, GetRandomUsersByOffset, offsets, sponsorID, startFrom, endUntil)
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
