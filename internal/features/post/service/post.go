package service

import (
	"context"
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
	// TODO
	return dto.PostItem{}, sharederrors.ErrNotImplemented
}

// GetPost returns a post if the caller is its author or the author's referral.
func (ps *PostService) GetPost(ctx context.Context, callerID, postID string) (dto.PostItem, error) {
	const op = "internal/features/post/service GetPost"
	// TODO: access check -> sharederrors.ErrPostAccessDenied
	return dto.PostItem{}, sharederrors.ErrNotImplemented
}

func (ps *PostService) UpdatePost(ctx context.Context, authorID, postID string, req dto.UpdatePostRequest) (dto.PostItem, error) {
	const op = "internal/features/post/service UpdatePost"
	// TODO
	return dto.PostItem{}, sharederrors.ErrNotImplemented
}

func (ps *PostService) DeletePost(ctx context.Context, authorID, postID string) error {
	const op = "internal/features/post/service DeletePost"
	// TODO
	return sharederrors.ErrNotImplemented
}

// Feed returns posts of the caller's current referrer.
func (ps *PostService) Feed(ctx context.Context, callerID string, params dto.PostListParams) (*dto.PostListResponse, error) {
	const op = "internal/features/post/service Feed"
	// TODO: no referrer -> sharederrors.ErrUserIsNotReferredYet
	return nil, sharederrors.ErrNotImplemented
}

// PostsOfUser returns posts of a user; empty username = caller's own posts.
func (ps *PostService) PostsOfUser(ctx context.Context, callerID, username string, params dto.PostListParams) (*dto.PostListResponse, error) {
	const op = "internal/features/post/service PostsOfUser"
	// TODO: access check -> sharederrors.ErrPostAccessDenied
	return nil, sharederrors.ErrNotImplemented
}
