package repo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/killerquinn/referral-system-go/internal/features/page/dto"
	sharederrors "github.com/killerquinn/referral-system-go/internal/shared/shared-errors.go"
)

const (
	pgUniqueViolation     = "23505"
	pgForeignKeyViolation = "23503"
	constraintPagePK      = "profile_pages_pkey"
)

type Repository struct {
	db *pgxpool.Pool
}

func New(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// pageTargets returns Scan destinations for pageColumns + mediaColumns.
func pageTargets(p *dto.PageItem) []any {
	t := []any{
		&p.Slug, &p.DisplayName, &p.Bio, &p.Settings, &p.IsPublished, &p.SlugChangedAt,
		&p.CreatedAt, &p.UpdatedAt, &p.BackgroundUploadID, &p.MusicUploadID,
	}
	t = append(t, p.Media.Avatar.Targets()...)
	t = append(t, p.Media.Background.Targets()...)
	return append(t, p.Media.Music.Targets()...)
}

func pgCode(err error) (code, constraint string) {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code, pgErr.ConstraintName
	}
	return "", ""
}

func (r *Repository) CreatePage(ctx context.Context, userID, slug string, displayName *string, settings dto.PageSettings) (dto.PageItem, error) {
	const op = "internal/features/page/repo CreatePage"

	if _, err := r.db.Exec(ctx, InsertPage, userID, slug, displayName, settings); err != nil {
		if code, constraint := pgCode(err); code == pgUniqueViolation {
			if constraint == constraintPagePK {
				return dto.PageItem{}, sharederrors.ErrPageAlreadyExists
			}
			return dto.PageItem{}, sharederrors.ErrSlugTaken
		}
		return dto.PageItem{}, fmt.Errorf("%s:%w", op, err)
	}

	return r.GetPageByUserID(ctx, userID)
}

func (r *Repository) GetPageByUserID(ctx context.Context, userID string) (dto.PageItem, error) {
	const op = "internal/features/page/repo GetPageByUserID"

	var p dto.PageItem
	if err := r.db.QueryRow(ctx, SelectPageByUserID, userID).Scan(pageTargets(&p)...); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return dto.PageItem{}, sharederrors.ErrPageNotFound
		}
		return dto.PageItem{}, fmt.Errorf("%s:%w", op, err)
	}
	return p, nil
}

func (r *Repository) GetPageBySlug(ctx context.Context, slug string) (dto.PageWithAuthor, error) {
	const op = "internal/features/page/repo GetPageBySlug"

	var p dto.PageWithAuthor
	targets := append(pageTargets(&p.PageItem), &p.UserID, &p.AuthorUsername, &p.OwnReferralKey, &p.AuthorBlocked)
	if err := r.db.QueryRow(ctx, SelectPageBySlug, slug).Scan(targets...); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return dto.PageWithAuthor{}, sharederrors.ErrPageNotFound
		}
		return dto.PageWithAuthor{}, fmt.Errorf("%s:%w", op, err)
	}
	return p, nil
}

// UpdatePage: nil = leave as is. backgroundUploadID / musicUploadID must be checked with
// IsUploadAttachable first; an upload deleted in between ends up as a FK violation -> ErrInvalidMediaAttachment.
func (r *Repository) UpdatePage(ctx context.Context, userID string, displayName, bio *string, settings *dto.PageSettings,
	isPublished *bool, backgroundUploadID, musicUploadID *string) (dto.PageItem, error) {
	const op = "internal/features/page/repo UpdatePage"

	res, err := r.db.Exec(ctx, UpdatePageByUserID, userID, displayName, bio, settings, isPublished, backgroundUploadID, musicUploadID)
	if err != nil {
		if code, _ := pgCode(err); code == pgForeignKeyViolation {
			return dto.PageItem{}, sharederrors.ErrInvalidMediaAttachment
		}
		return dto.PageItem{}, fmt.Errorf("%s:%w", op, err)
	}
	if res.RowsAffected() == 0 {
		return dto.PageItem{}, sharederrors.ErrPageNotFound
	}

	return r.GetPageByUserID(ctx, userID)
}

func (r *Repository) ChangeSlug(ctx context.Context, userID, newSlug string) error {
	const op = "internal/features/page/repo ChangeSlug"

	res, err := r.db.Exec(ctx, UpdatePageSlug, userID, newSlug)
	if err != nil {
		if code, _ := pgCode(err); code == pgUniqueViolation {
			return sharederrors.ErrSlugTaken
		}
		return fmt.Errorf("%s:%w", op, err)
	}
	if res.RowsAffected() == 0 {
		return sharederrors.ErrPageNotFound
	}
	return nil
}

func (r *Repository) IsSlugTaken(ctx context.Context, slug string) (bool, error) {
	const op = "internal/features/page/repo IsSlugTaken"

	var taken bool
	if err := r.db.QueryRow(ctx, SelectSlugExists, slug).Scan(&taken); err != nil {
		return false, fmt.Errorf("%s:%w", op, err)
	}
	return taken, nil
}

// IsUploadAttachable: the upload is the owner's, ready, public and has the given purpose.
func (r *Repository) IsUploadAttachable(ctx context.Context, uploadID, ownerID, purpose string) (bool, error) {
	const op = "internal/features/page/repo IsUploadAttachable"

	var ok bool
	if err := r.db.QueryRow(ctx, SelectUploadAttachable, uploadID, ownerID, purpose).Scan(&ok); err != nil {
		return false, fmt.Errorf("%s:%w", op, err)
	}
	return ok, nil
}

func (r *Repository) CountReferrals(ctx context.Context, authorID string) (int, error) {
	const op = "internal/features/page/repo CountReferrals"

	var n int
	if err := r.db.QueryRow(ctx, CountReferralsOfAuthor, authorID).Scan(&n); err != nil {
		return 0, fmt.Errorf("%s:%w", op, err)
	}
	return n, nil
}

func (r *Repository) GetRecentPostPreviews(ctx context.Context, authorID string, limit int) ([]dto.PostPreview, error) {
	const op = "internal/features/page/repo GetRecentPostPreviews"

	rows, err := r.db.Query(ctx, SelectRecentPostPreviews, authorID, limit)
	if err != nil {
		return nil, fmt.Errorf("%s:%w", op, err)
	}
	previews, err := pgx.CollectRows(rows, pgx.RowToStructByName[dto.PostPreview])
	if err != nil {
		return nil, fmt.Errorf("%s:%w", op, err)
	}
	return previews, nil
}

// GetViewerContext: sharederrors.ErrUserNotFound if the viewer doesn't exist anymore.
func (r *Repository) GetViewerContext(ctx context.Context, viewerID, authorID string) (dto.ViewerContext, error) {
	const op = "internal/features/page/repo GetViewerContext"

	var vc dto.ViewerContext
	if err := r.db.QueryRow(ctx, SelectViewerContext, viewerID, authorID).Scan(
		&vc.IsReferral, &vc.ReferredSince, &vc.HasOtherReferrer,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return dto.ViewerContext{}, sharederrors.ErrUserNotFound
		}
		return dto.ViewerContext{}, fmt.Errorf("%s:%w", op, err)
	}
	return vc, nil
}

// ===== views =====

func (r *Repository) GetPageForView(ctx context.Context, slug string) (dto.PageForView, error) {
	const op = "internal/features/page/repo GetPageForView"

	var p dto.PageForView
	if err := r.db.QueryRow(ctx, SelectPageForView, slug).Scan(&p.UserID, &p.IsPublished, &p.AuthorBlocked); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return dto.PageForView{}, sharederrors.ErrPageNotFound
		}
		return dto.PageForView{}, fmt.Errorf("%s:%w", op, err)
	}
	return p, nil
}

// RecordView counts the visitor once per day: the dedupe row and the +1 go in one transaction.
// Returns false if this visitor was already counted today.
func (r *Repository) RecordView(ctx context.Context, pageUserID string, day time.Time, visitorHash []byte) (bool, error) {
	const op = "internal/features/page/repo RecordView"

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("%s:%w", op, err)
	}
	defer tx.Rollback(ctx)

	res, err := tx.Exec(ctx, InsertViewVisitor, pageUserID, day, visitorHash)
	if err != nil {
		return false, fmt.Errorf("%s:%w", op, err)
	}
	if res.RowsAffected() == 0 {
		return false, nil // seen today
	}

	if _, err := tx.Exec(ctx, IncrementDailyViews, pageUserID, day); err != nil {
		return false, fmt.Errorf("%s:%w", op, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("%s:%w", op, err)
	}
	return true, nil
}

func (r *Repository) GetViewStats(ctx context.Context, pageUserID string, today time.Time) (dto.PageViews, error) {
	const op = "internal/features/page/repo GetViewStats"

	var v dto.PageViews
	if err := r.db.QueryRow(ctx, SelectViewStats, pageUserID, today).Scan(&v.Total, &v.Today); err != nil {
		return dto.PageViews{}, fmt.Errorf("%s:%w", op, err)
	}
	return v, nil
}

func (r *Repository) DeleteViewVisitorsBefore(ctx context.Context, day time.Time) (int, error) {
	const op = "internal/features/page/repo DeleteViewVisitorsBefore"

	res, err := r.db.Exec(ctx, DeleteOldViewVisitors, day)
	if err != nil {
		return 0, fmt.Errorf("%s:%w", op, err)
	}
	return int(res.RowsAffected()), nil
}
