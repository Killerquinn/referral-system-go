package service

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/killerquinn/referral-system-go/internal/features/referral/dto"
	httpfeatures "github.com/killerquinn/referral-system-go/internal/infrastructure/pkg/http-features"
	randfeatures "github.com/killerquinn/referral-system-go/internal/infrastructure/pkg/rand-features"
	sharederrors "github.com/killerquinn/referral-system-go/internal/shared/shared-errors.go"
	"go.uber.org/zap"
)

type RefService struct {
	log      *zap.Logger
	baseurl  string
	refCheck referralCheck
}

type referralCheck interface {
	CheckReferralByUsername(ctx context.Context, username string) (referrerusername string, referredSince time.Time, err error)
	GetReferralsByRefferersUsername(ctx context.Context, username string, cursor *time.Time, lastID *string, limit int) (entities []dto.ReferralItem, err error)
	GetReferralsByOwnUserID(ctx context.Context, username string, cursor *time.Time, lastID *string, limit int) (entities []dto.ReferralItem, err error)
	GetActualQuantityOfReferrers(ctx context.Context, userid string, startFrom *time.Time, endUntil *time.Time, verifiedOnly bool) (int, error)
	GetWinners(ctx context.Context, offsets []int, sponsorID string, startFrom *time.Time, endUntil *time.Time, verifiedOnly bool) ([]dto.WinnersDTO, error)
	GetDailyReferralCounts(ctx context.Context, ownerID string, since time.Time) ([]dto.DailyReferralCount, error)
}

func New(
	log *zap.Logger,
	baseurl string,
	referralCheck referralCheck,
) *RefService {
	return &RefService{
		log:      log,
		baseurl:  baseurl,
		refCheck: referralCheck,
	}
}

func (rfs *RefService) WhoseReferralUserIs(ctx context.Context, username string) (referrer string, referrerprofileURL string, referrerSince time.Time, err error) {
	const op = "/internal/features/referral/service WhoseReferralUserIs"

	log := rfs.log.With(
		zap.String("preparing to check whose referral user is: ", username),
		zap.String("op: ", op),
	)

	refusername, lastTimeReferred, err := rfs.refCheck.CheckReferralByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, sharederrors.ErrUserNotFound) {

			return "", "", time.Time{}, sharederrors.ErrUserNotFound
		}
		log.Error("err:", zap.Error(err))

		return "", "", time.Time{}, fmt.Errorf("%s%w", op, err)
	}

	return refusername, httpfeatures.BuildUrl(rfs.baseurl, refusername), lastTimeReferred, nil
}

func (rfs *RefService) CheckCurrentReferralList(ctx context.Context, referrerID string, params dto.ReferralListParams) (*dto.CurrentReferralListResponse, error) {
	const op = "/internal/features/referral/service CheckCurrentReferralList"

	log := rfs.log.With(
		zap.String("preparing to cursor based pagination to", referrerID),
		zap.String("op: ", op),
	)

	if params.Limit <= 0 {
		params.Limit = 20
	}
	if params.Limit > 100 {
		params.Limit = 100
	}

	var cursorTime *time.Time
	var cursorID *string

	var (
		entities []dto.ReferralItem
		err      error
	)

	if params.Cursor == "" {
		cursorTime = nil
		cursorID = nil
	} else {
		parsedTime, parsedID, err := httpfeatures.ParseCursor(params.Cursor)
		if err != nil {
			if errors.Is(err, sharederrors.ErrInvalidCursorFormat) {
				return nil, err
			}
			if errors.Is(err, sharederrors.ErrInvalidCursorTime) {
				return nil, err
			}
			return nil, fmt.Errorf("%s:%w", op, err)
		}
		cursorTime = &parsedTime
		cursorID = &parsedID
	}

	switch _, parseErr := uuid.Parse(referrerID); parseErr {
	case nil:
		log.Info("user tries to check his own referrals")
		entities, err = rfs.refCheck.GetReferralsByOwnUserID(ctx, referrerID, cursorTime, cursorID, params.Limit+1)
	default:
		log.Info("user tries to check other referrals by username referrals")
		entities, err = rfs.refCheck.GetReferralsByRefferersUsername(ctx, referrerID, cursorTime, cursorID, params.Limit+1)
	}

	if err != nil {
		switch {
		case errors.Is(err, sharederrors.ErrUserNotFound):
			return nil, sharederrors.ErrUserNotFound
		case errors.Is(err, sharederrors.ErrUserIsBanned):
			return nil, sharederrors.ErrUserIsBanned
		case errors.Is(err, sharederrors.ErrUserDoesntHaveReferralsYet):
			return nil, sharederrors.ErrUserDoesntHaveReferralsYet

		default:
			log.Error("failed to get referrals", zap.Error(err), zap.String("referrer_id", referrerID))
			return nil, fmt.Errorf("%s: %w", op, err)
		}
	}
	hasMore := false
	var nextCursor *string

	if len(entities) > params.Limit {
		hasMore = true
		entities = entities[:params.Limit]
		last := entities[len(entities)-1]
		raw := fmt.Sprintf("%s|%s", last.JoinedAt.Format(time.RFC3339Nano), last.ID)
		enc := base64.RawURLEncoding.EncodeToString([]byte(raw))
		nextCursor = &enc
	}
	items := make([]dto.ReferralItem, 0, len(entities))
	for _, e := range entities {
		items = append(items, dto.ReferralItem{
			Username:   e.Username,
			ProfileURL: httpfeatures.BuildUrl(rfs.baseurl, e.Username),
			JoinedAt:   e.JoinedAt,
		})
	}

	return &dto.CurrentReferralListResponse{
		Items:      items,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	}, nil

}

func (rfs *RefService) StartContest(ctx context.Context, sponsorID string, winnersQuantity int, startFrom *time.Time, endUntil *time.Time, verifiedOnly bool) (dto.ContestBetweenReferralsResponse, error) {
	const op = "internal/features/referral/service - StartContest"

	log := rfs.log.With(
		zap.String("sponsor_id", sponsorID),
		zap.String("op", op),
	)

	actualNumberOfReferrers, err := rfs.refCheck.GetActualQuantityOfReferrers(ctx, sponsorID, startFrom, endUntil, verifiedOnly)
	if err != nil {
		if errors.Is(err, sharederrors.ErrInsufficientQuantityOfReferrals) {
			return dto.ContestBetweenReferralsResponse{}, sharederrors.ErrInsufficientQuantityOfReferrals
		}
		log.Error("failed while tried to check actual quantity of users referrals", zap.Error(err))
		return dto.ContestBetweenReferralsResponse{}, fmt.Errorf("%s:%w", op, err)
	}
	if actualNumberOfReferrers == 0 {
		return dto.ContestBetweenReferralsResponse{}, sharederrors.ErrInsufficientQuantityOfReferrals
	}
	if winnersQuantity > actualNumberOfReferrers {
		winnersQuantity = actualNumberOfReferrers
	}

	//To-Do: cache of actual number of referrers needed there

	offset, err := randfeatures.GenerateRandOffset(winnersQuantity, actualNumberOfReferrers)
	if err != nil {
		log.Error("unnable to generate random offset", zap.Error(err))

		return dto.ContestBetweenReferralsResponse{}, fmt.Errorf("%s:%w", op, err)
	}

	winners, err := rfs.refCheck.GetWinners(ctx, offset, sponsorID, startFrom, endUntil, verifiedOnly)
	if err != nil {
		log.Error("unnable to get winners", zap.Error(err))

		return dto.ContestBetweenReferralsResponse{}, fmt.Errorf("%s:%w", op, err)
	}

	calculatedWinners := make([]dto.WinnersDTO, len(winners))

	for i, w := range winners {
		calculatedWinners[i] = dto.WinnersDTO{
			Place:         i + 1,
			Username:      w.Username,
			UserURL:       httpfeatures.BuildUrl(rfs.baseurl, w.Username),
			ReferralSince: w.ReferralSince,
		}
	}

	return dto.ContestBetweenReferralsResponse{
		Winners: calculatedWinners,
		Message: "users raffled successfully",
	}, nil
}

// ReferralStats builds a per-day series for the last `days` days (today included, UTC).
// The repo only returns days that had referrals; the gaps are filled with zeros here,
// so the chart always gets exactly `days` points.
func (rfs *RefService) ReferralStats(ctx context.Context, ownerID string, days int) (*dto.ReferralStatsResponse, error) {
	const op = "internal/features/referral/service - ReferralStats"

	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	since := today.AddDate(0, 0, -(days - 1))

	rows, err := rfs.refCheck.GetDailyReferralCounts(ctx, ownerID, since)
	if err != nil {
		rfs.log.Error("failed to get daily referral counts", zap.Error(err), zap.String("owner_id", ownerID), zap.String("op", op))
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	const layout = "2006-01-02"
	byDay := make(map[string]int, len(rows))
	for _, row := range rows {
		byDay[row.Day.UTC().Format(layout)] += row.Count
	}

	points := make([]dto.ReferralStatsPoint, 0, days)
	total := 0
	for d := since; !d.After(today); d = d.AddDate(0, 0, 1) {
		key := d.Format(layout)
		c := byDay[key]
		total += c
		points = append(points, dto.ReferralStatsPoint{Date: key, Count: c})
	}

	return &dto.ReferralStatsResponse{
		Days:    days,
		Total:   total,
		Points:  points,
		Message: "ok",
	}, nil
}
