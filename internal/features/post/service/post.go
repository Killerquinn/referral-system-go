package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/killerquinn/referral-system-go/internal/features/post/dto"
	sharederrors "github.com/killerquinn/referral-system-go/internal/shared/shared-errors.go"
	"go.uber.org/zap"
)

type postStorage interface {
	CreatePost(ctx context.Context, authorID, title, body string) (dto.PostItem, error)
	GetPostByID(ctx context.Context, postID string) (dto.PostItem, error)
	UpdatePost(ctx context.Context, postID, authorID string, title, body *string) (dto.PostItem, error)
	DeletePost(ctx context.Context, postID, authorID string) error
	GetPostsByAuthorID(ctx context.Context, authorID string, cursor *time.Time, lastID *string, limit int) ([]dto.PostItem, error)
	GetPostsByAuthorUsername(ctx context.Context, username string, cursor *time.Time, lastID *string, limit int) ([]dto.PostItem, error)
	CanReadPostsOf(ctx context.Context, readerID, authorUsername string) (bool, error)
}

type PostService struct {
	log     *zap.Logger
	baseurl string
	storage postStorage
}

func New(log *zap.Logger, baseurl string, storage postStorage) *PostService {
	return &PostService{
		log:     log,
		baseurl: baseurl,
		storage: storage,
	}
}

func (ps *PostService) CreatePost(ctx context.Context, authorID string, req dto.CreatePostRequest) (dto.PostItem, error) {
	const op = "internal/features/post/service CreatePost"

	log := ps.log.With(
		zap.String("op", op),
		zap.String("user_id", authorID),
	)

	item, err := ps.storage.CreatePost(ctx, authorID, req.Title, req.Body)
	if err != nil {
		log.Error("error", zap.Error(err))
		return dto.PostItem{}, fmt.Errorf("%s:%w", op, err)
	}

	return item, sharederrors.ErrNotImplemented
}

// GetPost returns a post if the caller is its author or the author's referral.
func (ps *PostService) GetPost(ctx context.Context, callerID, postID string) (dto.PostItem, error) {
	const op = "internal/features/post/service GetPost"

	log := ps.log.With(
		zap.String("op", op),
		zap.String("user_id", callerID),
	)

	post, err := ps.storage.GetPostByID(ctx, postID)
	if err != nil {
		if errors.Is(err, sharederrors.ErrPostNotFound) {
			return dto.PostItem{}, sharederrors.ErrPostNotFound
		}
		if errors.Is(err, sharederrors.ErrUserIsBanned) {
			return dto.PostItem{}, sharederrors.ErrUserIsBanned
		}
		log.Error("error", zap.Error(err))
		return dto.PostItem{}, fmt.Errorf("%s:%w", op, err)
	}
	// TODO: access check -> sharederrors.ErrPostAccessDenied

	able, err := ps.storage.CanReadPostsOf(ctx, callerID, post.AuthorID)
	if err != nil {
		return dto.PostItem{}, fmt.Errorf("%s:%w", op, err)
	}
	if !able {
		return dto.PostItem{}, sharederrors.ErrPostAccessDenied
	}

	return post, err
}

func (ps *PostService) UpdatePost(ctx context.Context, authorID, postID string, req dto.UpdatePostRequest) (dto.PostItem, error) {
	const op = "internal/features/post/service UpdatePost"
	// TODO

	log := ps.log.With(
		zap.String("op", op),
		zap.String("user_id", authorID),
	)

	updatedPost, err := ps.storage.UpdatePost(ctx, postID, authorID, req.Title, req.Body)
	if err != nil {
		log.Error("err", zap.Error(err))
		return dto.PostItem{}, fmt.Errorf("%s:%w", op, err)
	}

	return updatedPost, err
}

func (ps *PostService) DeletePost(ctx context.Context, authorID, postID string) error {
	const op = "internal/features/post/service DeletePost"
	// TODO

	log := ps.log.With(
		zap.String("op", op),
		zap.String("user_id", authorID),
	)

	err := ps.storage.DeletePost(ctx, postID, authorID)
	if err != nil {
		if errors.Is(err, sharederrors.ErrPostNotFound) {
			return sharederrors.ErrPostNotFound
		}
		log.Error("err", zap.Error(err))

		return fmt.Errorf("%s:%w", op, err)
	}

	return err
}

// Feed returns posts of the caller's current referrer.
func (ps *PostService) Feed(ctx context.Context, callerID string, params dto.PostListParams) (*dto.PostListResponse, error) {
	const op = "internal/features/post/service Feed"
	// TODO: no referrer -> sharederrors.ErrUserIsNotReferredYet

	log := ps.log.With(
		zap.String("op", op),
		zap.String("user_id", authorID),
	)

	return nil, sharederrors.ErrNotImplemented
}

// PostsOfUser returns posts of a user; empty username = caller's own posts.
func (ps *PostService) PostsOfUser(ctx context.Context, callerID, username string, params dto.PostListParams) (*dto.PostListResponse, error) {
	const op = "internal/features/post/service PostsOfUser"
	// TODO: access check -> sharederrors.ErrPostAccessDenied

	log := ps.log.With(
		zap.String("op", op),
		zap.String("user_id", authorID),
	)

	return nil, sharederrors.ErrNotImplemented
}
