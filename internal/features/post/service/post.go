package service

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/killerquinn/referral-system-go/internal/features/post/dto"
	httpfeatures "github.com/killerquinn/referral-system-go/internal/infrastructure/pkg/http-features"
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
	GetFeed(ctx context.Context, callerID string, cursor *time.Time, lastPostID *string, limit int) ([]dto.PostItem, error)
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
		zap.String("user_id", callerID),
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
		entities []dto.PostItem
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

			log.Error("err", zap.Error(err))
			return nil, fmt.Errorf("%s:%w", op, err)
		}
		cursorTime = &parsedTime
		cursorID = &parsedID

	}

	entities, err = ps.storage.GetFeed(ctx, callerID, cursorTime, cursorID, params.Limit+1)
	if err != nil {
		if errors.Is(err, sharederrors.ErrUserIsNotReferredYet) {
			return nil, err
		}
		if errors.Is(err, sharederrors.ErrUserIsBanned) {
			return nil, err
		}
		log.Error("err", zap.Error(err))

		return nil, fmt.Errorf("%s:%w", op, err)
	}

	hasMore := false
	var nextCursor *string

	if len(entities) > params.Limit {
		hasMore = true
		entities = entities[:params.Limit]
		last := entities[len(entities)-1]
		raw := fmt.Sprintf("%s|%s", last.CreatedAt.Format(time.RFC3339Nano), last.ID)
		enc := base64.RawURLEncoding.EncodeToString([]byte(raw))
		nextCursor = &enc
	}

	items := make([]dto.PostItem, 0, len(entities))
	for _, e := range entities {
		items = append(items, dto.PostItem{
			ID:             e.ID,
			AuthorUsername: e.AuthorUsername,
			AuthorURL:      e.AuthorURL,
			Title:          e.Title,
			Body:           e.Body,
			CreatedAt:      e.CreatedAt,
			UpdatedAt:      e.UpdatedAt,
		})
	}

	return &dto.PostListResponse{
		Items:      items,
		NextCursor: nextCursor,
		HasMore:    hasMore,
		Message:    "",
	}, err
}

// PostsOfUser returns posts of a user; empty username = caller's own posts.
func (ps *PostService) PostsOfUser(ctx context.Context, callerID, username string, params dto.PostListParams) (*dto.PostListResponse, error) {
	const op = "internal/features/post/service PostsOfUser"
	// TODO: access check -> sharederrors.ErrPostAccessDenied

	log := ps.log.With(
		zap.String("op", op),
		zap.String("user_id", callerID),
	)
	var check_own bool

	if callerID != username {
		able, err := ps.storage.CanReadPostsOf(ctx, callerID, username)
		if err != nil {
			if errors.Is(err, sharederrors.ErrUserNotFound) {
				return nil, err
			}
			return nil, fmt.Errorf("%s:%w", op, err)
		}
		if !able {
			return nil, sharederrors.ErrPostAccessDenied
		}
	} else {
		check_own = true
	}

	if params.Limit <= 0 {
		params.Limit = 20
	}

	if params.Limit > 100 {
		params.Limit = 100
	}

	var cursorTime *time.Time
	var cursorID *string

	var (
		entities []dto.PostItem
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

			log.Error("err", zap.Error(err))
			return nil, fmt.Errorf("%s:%w", op, err)
		}
		cursorTime = &parsedTime
		cursorID = &parsedID
	}

	switch check_own {
	case true:
		entities, err = ps.storage.GetPostsByAuthorID(ctx, callerID, cursorTime, cursorID, params.Limit)
		if err != nil {
			log.Error("err", zap.Error(err))

			return nil, fmt.Errorf("%s:%w", op, err)
		}
	default:
		entities, err = ps.storage.GetPostsByAuthorUsername(ctx, username, cursorTime, cursorID, params.Limit)
		if err != nil {
			if errors.Is(err, sharederrors.ErrUserIsBanned) {
				return nil, err
			}
			log.Error("err", zap.Error(err))

			return nil, fmt.Errorf("%s:%w", op, err)
		}
	}

	hasMore := false
	var nextCursor *string

	if len(entities) > params.Limit {
		hasMore = true
		entities = entities[:params.Limit]
		last := entities[len(entities)-1]
		raw := fmt.Sprintf("%s|%s", last.CreatedAt.Format(time.RFC3339Nano), last.ID)
		enc := base64.RawURLEncoding.EncodeToString([]byte(raw))
		nextCursor = &enc
	}
	items := make([]dto.PostItem, 0, len(entities))
	for _, e := range entities {
		items = append(items, dto.PostItem{
			ID:             e.ID,
			AuthorUsername: e.AuthorUsername,
			AuthorURL:      e.AuthorURL, //add url builder by username
			Title:          e.Title,
			Body:           e.Body,
			CreatedAt:      e.CreatedAt,
			UpdatedAt:      e.UpdatedAt,
		})
	}

	return &dto.PostListResponse{
		Items:      items,
		NextCursor: nextCursor,
		HasMore:    hasMore,
		Message:    "",
	}, err
}
