package repo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/killerquinn/referral-system-go/internal/features/post/dto"
	vidembed "github.com/killerquinn/referral-system-go/internal/shared/embed"
	sharederrors "github.com/killerquinn/referral-system-go/internal/shared/shared-errors.go"
)

type Repository struct {
	db *pgxpool.Pool
}

func New(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) CreatePost(ctx context.Context, authorID, title, body string, mediaIDs []string, embeds []vidembed.Embed) (dto.PostItem, error) {
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

	if err := insertEmbeds(ctx, tx, postItem.ID, embeds); err != nil {
		return dto.PostItem{}, fmt.Errorf("%s:%w", op, err)
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
// Returns the upload ids that were attached BEFORE the change (nil when media wasn't touched).
// Deletes all video embeds and replaces with new if user already has ones, or completely deletes them
// When embeds != nil, the list of video embeds is replaced the same way (an empty slice removes them all).
func (r *Repository) UpdatePost(ctx context.Context, postID, authorID string, title, body *string, mediaIDs *[]string, embeds *[]vidembed.Embed) ([]string, error) {
	const op = "internal/features/post/repo UpdatePost"

	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("%s:%w", op, err)
	}
	defer tx.Rollback(ctx)

	// Also the ownership check: author_id is in the WHERE. The UPDATE locks the post row, so two
	// concurrent edits of the same post run one after another instead of mixing their media lists.
	res, err := tx.Exec(ctx, UpdatePostByIDAndAuthor, postID, authorID, title, body)
	if err != nil {
		return nil, fmt.Errorf("%s:%w", op, err)
	}
	if res.RowsAffected() == 0 {
		return nil, sharederrors.ErrPostNotFound
	}

	var previous []string
	if mediaIDs != nil {
		// delete + insert instead of moving rows around: UNIQUE (post_id, position) would
		// conflict on every step of a reorder
		previous, err = unlinkMedia(ctx, tx, postID)
		if err != nil {
			return nil, fmt.Errorf("%s:%w", op, err)
		}

		if ids := *mediaIDs; len(ids) > 0 {
			res, err := tx.Exec(ctx, InsertMedia, postID, authorID, ids)
			if err != nil {
				return nil, fmt.Errorf("%s:%w", op, err)
			}
			if res.RowsAffected() != int64(len(ids)) {
				return nil, sharederrors.ErrInvalidMedia
			}
		}
	}

	if embeds != nil {
		_, err := tx.Exec(ctx, DeletePostEmbeds, postID)
		if err != nil {
			return nil, fmt.Errorf("%s:%w", op, err)
		}

		if err := insertEmbeds(ctx, tx, postID, *embeds); err != nil {
			return nil, fmt.Errorf("%s:%w", op, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("%s:%w", op, err)
	}

	return previous, nil
}

func (r *Repository) LoadEmbeds(ctx context.Context, postIDs []string) (map[string][]vidembed.Embed, error) {
	const op = "internal/features/post/repo LoadEmbeds"

	if len(postIDs) == 0 {
		return map[string][]vidembed.Embed{}, nil
	}

	rows, err := r.db.Query(ctx, ReadEmbeds, postIDs)
	if err != nil {
		return nil, fmt.Errorf("%s:%w", op, err)
	}
	defer rows.Close()

	res := make(map[string][]vidembed.Embed, len(postIDs))

	for rows.Next() {
		var postID string
		var e vidembed.Embed
		if err := rows.Scan(&postID, &e.Provider, &e.ExternalID); err != nil {
			return nil, fmt.Errorf("%s:%w", op, err)
		}
		res[postID] = append(res[postID], e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s:%w", op, err)
	}
	return res, nil
}

// unlinkMedia removes all attachment links of a post and returns the upload ids they pointed to.
func unlinkMedia(ctx context.Context, tx pgx.Tx, postID string) ([]string, error) {
	rows, err := tx.Query(ctx, DeletePostMedia, postID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}

func insertEmbeds(ctx context.Context, tx pgx.Tx, postID string, embeds []vidembed.Embed) error {
	const op = "internal/features/post/rep insertEmbeds"

	if len(embeds) == 0 {
		return nil
	}

	ids := make([]string, len(embeds))
	providers := make([]string, len(embeds))

	for i := range embeds {
		ids[i] = embeds[i].ExternalID
		providers[i] = embeds[i].Provider
	}

	if _, err := tx.Exec(ctx, InsertEmbeds, postID, providers, ids); err != nil {
		return fmt.Errorf("%s:%w", op, err)
	}

	return nil
}

// DeletePost deletes the author's post and returns the upload ids that were attached to it,
// so the caller can delete the files (the links themselves would go away by ON DELETE CASCADE,
// but then we wouldn't know which files to clean up).
func (r *Repository) DeletePost(ctx context.Context, postID, authorID string) ([]string, error) {
	const op = "internal/features/post/repo DeletePost"

	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("%s:%w", op, err)
	}
	defer tx.Rollback(ctx)

	// ownership check + row lock first
	var id string
	if err := tx.QueryRow(ctx, LockAuthorPost, postID, authorID).Scan(&id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, sharederrors.ErrPostNotFound
		}
		return nil, fmt.Errorf("%s:%w", op, err)
	}

	uploadIDs, err := unlinkMedia(ctx, tx, postID)
	if err != nil {
		return nil, fmt.Errorf("%s:%w", op, err)
	}

	if _, err := tx.Exec(ctx, DeletePostByIDAndAuthor, postID, authorID); err != nil {
		return nil, fmt.Errorf("%s:%w", op, err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("%s:%w", op, err)
	}

	return uploadIDs, nil
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
