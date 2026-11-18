package repo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/killerquinn/referral-system-go/internal/features/post/dto"
	sharederrors "github.com/killerquinn/referral-system-go/internal/shared/shared-errors.go"
)

type Repository struct {
	db *pgxpool.Pool
}

func New(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) CreatePost(ctx context.Context, authorID, title, body string, mediaIDs []string) (dto.PostItem, error) {
	const op = "internal/features/post/repo CreatePost"

	var postItem dto.PostItem

	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return dto.PostItem{}, fmt.Errorf("%s:%w", op, err)
	}
	defer tx.Rollback(ctx)

	if err := tx.QueryRow(ctx, InsertPost, authorID, title, body).Scan(
		&postItem.ID,
		&postItem.AuthorID,
		&postItem.AuthorUsername,
		&postItem.AuthorAvatarType,
		&postItem.AuthorAvatarKey,
		&postItem.AuthorAvatarThumb,
		&postItem.AuthorAvatarBlurhash,
		&postItem.Title,
		&postItem.Body,
		&postItem.CreatedAt,
	); err != nil {
		return dto.PostItem{}, fmt.Errorf("%s:%w", op, err)
	}

	if len(mediaIDs) > 0 {
		res, err := tx.Exec(ctx, InsertMedia, postItem.ID, authorID, mediaIDs)
		if err != nil {
			return dto.PostItem{}, fmt.Errorf("%s:%w", op, err)
		}
		if res.RowsAffected() != int64(len(mediaIDs)) {
			return dto.PostItem{}, sharederrors.ErrInvalidMedia
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return dto.PostItem{}, fmt.Errorf("%s:%w", op, err)
	}

	return postItem, nil
}

func (r *Repository) LoadMedia(ctx context.Context, postIDs []string) (map[string][]dto.PostMediaRow, error) {
	const op = "internal/features/post/repo LoadMedia"

	if len(postIDs) == 0 {
		return map[string][]dto.PostMediaRow{}, nil
	}

	rows, err := r.db.Query(ctx, ReadMedia, postIDs)
	if err != nil {
		return nil, fmt.Errorf("%s:%w", op, err)
	}
	defer rows.Close()

	list, err := pgx.CollectRows(rows, pgx.RowToStructByPos[dto.PostMediaRow])
	if err != nil {
		return nil, fmt.Errorf("%s:%w", op, err)
	}

	res := make(map[string][]dto.PostMediaRow)
	for _, mediaUnit := range list {
		res[mediaUnit.PostID] = append(res[mediaUnit.PostID], mediaUnit)
	}

	return res, nil
}

func (r *Repository) GetPostByID(ctx context.Context, postID string) (dto.PostItem, error) {
	const op = "internal/features/post/repo GetPostByID"

	var postItem dto.PostItem
	var banned bool

	if err := r.db.QueryRow(ctx, SelectPostByID, postID).Scan(
		&postItem.ID,
		&postItem.AuthorID,
		&postItem.AuthorUsername,
		&postItem.AuthorAvatarType,
		&postItem.AuthorAvatarKey,
		&postItem.AuthorAvatarThumb,
		&postItem.AuthorAvatarBlurhash,
		&postItem.Title,
		&postItem.Body,
		&postItem.CreatedAt,
		&postItem.UpdatedAt,
		&banned,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return dto.PostItem{}, sharederrors.ErrPostNotFound
		}
		return dto.PostItem{}, fmt.Errorf("%s:%w", op, err)
	}

	if banned {
		return dto.PostItem{}, sharederrors.ErrUserIsBanned
	}

	return postItem, nil
}

// UpdatePost changes title/body (nil = keep) and, when mediaIDs != nil, replaces the whole list of
// attachments (an empty slice removes them all). Everything happens in one transaction.
func (r *Repository) UpdatePost(ctx context.Context, postID, authorID string, title, body *string, mediaIDs *[]string) error {
	const op = "internal/features/post/repo UpdatePost"

	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("%s:%w", op, err)
	}
	defer tx.Rollback(ctx)

	// Also the ownership check: author_id is in the WHERE. The UPDATE locks the post row, so two
	// concurrent edits of the same post run one after another instead of mixing their media lists.
	res, err := tx.Exec(ctx, UpdatePostByIDAndAuthor, postID, authorID, title, body)
	if err != nil {
		return fmt.Errorf("%s:%w", op, err)
	}
	if res.RowsAffected() == 0 {
		return sharederrors.ErrPostNotFound
	}

	if mediaIDs != nil {
		// delete + insert instead of moving rows around: UNIQUE (post_id, position) would
		// conflict on every step of a reorder
		if _, err := tx.Exec(ctx, DeletePostMedia, postID); err != nil {
			return fmt.Errorf("%s:%w", op, err)
		}

		if ids := *mediaIDs; len(ids) > 0 {
			res, err := tx.Exec(ctx, InsertMedia, postID, authorID, ids)
			if err != nil {
				return fmt.Errorf("%s:%w", op, err)
			}
			if res.RowsAffected() != int64(len(ids)) {
				return sharederrors.ErrInvalidMedia
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("%s:%w", op, err)
	}

	return nil
}

func (r *Repository) DeletePost(ctx context.Context, postID, authorID string) error {
	const op = "internal/features/post/repo DeletePost"

	res, err := r.db.Exec(ctx, DeletePostByIDAndAuthor, postID, authorID)
	if err != nil {
		return fmt.Errorf("%s:%w", op, err)
	}
	if res.RowsAffected() == 0 {
		return sharederrors.ErrPostNotFound
	}

	return nil
}

func (r *Repository) GetPostsByAuthorID(ctx context.Context, authorID string, cursor *time.Time, lastID *string, limit int) ([]dto.PostItem, error) {
	const op = "internal/features/post/repo GetPostsByAuthorID"

	items, err := r.queryPosts(ctx, SelectPostsByAuthorID, authorID, cursor, lastID, limit)
	if err != nil {
		return nil, fmt.Errorf("%s:%w", op, err)
	}

	return items, nil
}

func (r *Repository) GetPostsByAuthorUsername(ctx context.Context, username string, cursor *time.Time, lastID *string, limit int) ([]dto.PostItem, error) {
	const op = "internal/features/post/repo GetPostsByAuthorUsername"

	var banned bool
	if err := r.db.QueryRow(ctx, SelectAuthorBlockedByUsername, username).Scan(&banned); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, sharederrors.ErrUserNotFound
		}
		return nil, fmt.Errorf("%s:%w", op, err)
	}
	if banned {
		return nil, sharederrors.ErrUserIsBanned
	}

	items, err := r.queryPosts(ctx, SelectPostsByAuthorUsername, username, cursor, lastID, limit)
	if err != nil {
		return nil, fmt.Errorf("%s:%w", op, err)
	}

	return items, nil
}

func (r *Repository) CanReadPostsOf(ctx context.Context, readerID, authorUsername string) (bool, error) {
	const op = "internal/features/post/repo CanReadPostsOf"

	var canRead bool
	if err := r.db.QueryRow(ctx, CanReadPostsOf, readerID, authorUsername).Scan(&canRead); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, sharederrors.ErrUserNotFound
		}
		return false, fmt.Errorf("%s:%w", op, err)
	}

	return canRead, nil
}

// GetFeed returns posts of the caller's current referrer (users.referred_by_id).
func (r *Repository) GetFeed(ctx context.Context, callerID string, cursor *time.Time, lastPostID *string, limit int) ([]dto.PostItem, error) {
	const op = "internal/features/post/repo GetFeed"

	var (
		referrerID *string
		banned     bool
	)
	if err := r.db.QueryRow(ctx, SelectCurrentReferrer, callerID).Scan(&referrerID, &banned); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, sharederrors.ErrUserNotFound
		}
		return nil, fmt.Errorf("%s:%w", op, err)
	}
	if referrerID == nil {
		return nil, sharederrors.ErrUserIsNotReferredYet
	}
	if banned {
		return nil, sharederrors.ErrUserIsBanned
	}

	items, err := r.queryPosts(ctx, SelectPostsByAuthorID, *referrerID, cursor, lastPostID, limit)
	if err != nil {
		return nil, fmt.Errorf("%s:%w", op, err)
	}

	return items, nil
}

// queryPosts runs one of the paginated list queries ($1 key, $2 cursor time, $3 cursor id, $4 limit).
// Empty result is not an error: returns an empty slice.
func (r *Repository) queryPosts(ctx context.Context, query, key string, cursor *time.Time, lastID *string, limit int) ([]dto.PostItem, error) {
	rows, err := r.db.Query(ctx, query, key, cursor, lastID, limit)
	if err != nil {
		return nil, err
	}

	return pgx.CollectRows(rows, pgx.RowToStructByNameLax[dto.PostItem])
}
