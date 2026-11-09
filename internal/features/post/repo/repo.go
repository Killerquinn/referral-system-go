package repo

import (
	"context"
	"time"

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

func (r *Repository) CreatePost(ctx context.Context, authorID, title, body string) (dto.PostItem, error) {
	const op = "internal/features/post/repo CreatePost"
	// TODO
	return dto.PostItem{}, sharederrors.ErrNotImplemented
}

func (r *Repository) GetPostByID(ctx context.Context, postID string) (dto.PostItem, error) {
	const op = "internal/features/post/repo GetPostByID"
	// TODO: pgx.ErrNoRows -> sharederrors.ErrPostNotFound
	return dto.PostItem{}, sharederrors.ErrNotImplemented
}

func (r *Repository) UpdatePost(ctx context.Context, postID, authorID string, title, body *string) (dto.PostItem, error) {
	const op = "internal/features/post/repo UpdatePost"
	// TODO: 0 rows affected -> sharederrors.ErrPostNotFound
	return dto.PostItem{}, sharederrors.ErrNotImplemented
}

func (r *Repository) DeletePost(ctx context.Context, postID, authorID string) error {
	const op = "internal/features/post/repo DeletePost"
	// TODO: 0 rows affected -> sharederrors.ErrPostNotFound
	return sharederrors.ErrNotImplemented
}

func (r *Repository) GetPostsByAuthorID(ctx context.Context, authorID string, cursor *time.Time, lastID *string, limit int) ([]dto.PostItem, error) {
	const op = "internal/features/post/repo GetPostsByAuthorID"
	// TODO
	return nil, sharederrors.ErrNotImplemented
}

func (r *Repository) GetPostsByAuthorUsername(ctx context.Context, username string, cursor *time.Time, lastID *string, limit int) ([]dto.PostItem, error) {
	const op = "internal/features/post/repo GetPostsByAuthorUsername"
	// TODO
	return nil, sharederrors.ErrNotImplemented
}

func (r *Repository) CanReadPostsOf(ctx context.Context, readerID, authorUsername string) (bool, error) {
	const op = "internal/features/post/repo CanReadPostsOf"
	// TODO: unknown author -> sharederrors.ErrUserNotFound
	return false, sharederrors.ErrNotImplemented
}

func (r *Repository) GetFeed(ctx context.Context, callerID string, cursor *time.Time, lastPostID *string, limit int) ([]dto.PostItem, error) {
	const op = "internal/features/post/repo GetFeed"

	return nil, sharederrors.ErrNotImplemented
}
