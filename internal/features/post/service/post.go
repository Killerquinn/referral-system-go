package service

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/killerquinn/referral-system-go/internal/features/post/dto"
	httpfeatures "github.com/killerquinn/referral-system-go/internal/infrastructure/pkg/http-features"
	vidembed "github.com/killerquinn/referral-system-go/internal/shared/embed"
	"github.com/killerquinn/referral-system-go/internal/shared/media"
	sharederrors "github.com/killerquinn/referral-system-go/internal/shared/shared-errors.go"
	"go.uber.org/zap"
)

type postStorage interface {
	CreatePost(ctx context.Context, authorID, title, body string, mediaIDs []string, embeds []vidembed.Embed) (dto.PostItem, error)
	GetPostByID(ctx context.Context, postID string) (dto.PostItem, error)
	UpdatePost(ctx context.Context, postID, authorID string, title, body *string, mediaIDs *[]string, embeds *[]vidembed.Embed) ([]string, error)
	DeletePost(ctx context.Context, postID, authorID string) ([]string, error)
	GetPostsByAuthorID(ctx context.Context, authorID string, cursor *time.Time, lastID *string, limit int) ([]dto.PostItem, error)
	GetPostsByAuthorUsername(ctx context.Context, username string, cursor *time.Time, lastID *string, limit int) ([]dto.PostItem, error)
	CanReadPostsOf(ctx context.Context, readerID, authorUsername string) (bool, error)
	GetFeed(ctx context.Context, callerID string, cursor *time.Time, lastPostID *string, limit int) ([]dto.PostItem, error)
	LoadMedia(ctx context.Context, postIDs []string) (map[string][]dto.PostMediaRow, error)
	LoadEmbeds(ctx context.Context, postIDs []string) (map[string][]vidembed.Embed, error)
}

// uploadDeleter removes an upload completely (row, files in R2, quota). *uploadservice.UploadService.
type uploadDeleter interface {
	DeleteUpload(ctx context.Context, userID, uploadID string) error
}

type MediaSigner interface {
	PrivateBucket() string
	PresignGet(ctx context.Context, bucket, key string) (string, time.Time, error)
}

type PostService struct {
	log     *zap.Logger
	baseurl string
	storage postStorage
	urls    media.URLBuilder // nil when R2 is disabled -> no avatar URLs
	signer  MediaSigner
	uploads uploadDeleter
}

const ()

func New(log *zap.Logger, baseurl string, storage postStorage, urls media.URLBuilder, signer MediaSigner, uploads uploadDeleter) *PostService {
	return &PostService{
		log:     log,
		baseurl: baseurl,
		storage: storage,
		urls:    urls,
		signer:  signer,
		uploads: uploads,
	}
}

// deleteUploads frees files that are no longer attached to anything. Runs after the transaction
// committed; a failure only leaves garbage in storage, so it's logged, not returned.
// A file still attached to another post can't be deleted (post_uploads.upload_id is ON DELETE RESTRICT).
func (ps *PostService) deleteUploads(ctx context.Context, log *zap.Logger, authorID string, ids []string) {
	if ps.uploads == nil {
		return
	}
	for _, id := range ids {
		if err := ps.uploads.DeleteUpload(context.WithoutCancel(ctx), authorID, id); err != nil {
			log.Warn("unable to delete a detached post file", zap.String("upload_id", id), zap.Error(err))
		}
	}
}

func parseEmbeds(links []string) ([]vidembed.Embed, error) {
	if len(links) > 4 {
		return nil, sharederrors.ErrTooManyEmbeds
	}

	seen := map[vidembed.Embed]struct{}{}
	parsedLinks := make([]vidembed.Embed, len(links))
	for i, l := range links {
		parsedLink, err := vidembed.Parse(l)
		if err != nil {
			return nil, sharederrors.ErrInvalidEmbed
		}

		if _, ok := seen[parsedLink]; ok {
			return nil, sharederrors.ErrDuplicatedEmbed
		}
		seen[parsedLink] = struct{}{}
		parsedLinks[i] = parsedLink
	}
	return parsedLinks, nil
}

func (ps *PostService) attachEmbeds(ctx context.Context, items []dto.PostItem) error {
	const op = "internal/features/post/service attachEmbeds"

	if len(items) == 0 {
		return nil
	}

	ids := make([]string, len(items))
	for i := range items {
		ids[i] = items[i].ID
	}
	m, err := ps.storage.LoadEmbeds(ctx, ids)
	if err != nil {
		return fmt.Errorf("%s:%w", op, err)
	}

	for i := range items {
		e := m[items[i].ID]
		if e == nil {
			e = make([]vidembed.Embed, 0)
		}
		items[i].Embeds = e
	}
	return nil
}

// detached returns ids from before that are not in after.
func detached(before []string, after []string) []string {
	keep := make(map[string]struct{}, len(after))
	for _, id := range after {
		keep[id] = struct{}{}
	}
	var gone []string
	for _, id := range before {
		if _, ok := keep[id]; !ok {
			gone = append(gone, id)
		}
	}
	return gone
}

func (ps *PostService) getPostsWithMedia(ctx context.Context, items []dto.PostItem) error {
	const op = "internal/features/post/service getPostsWithMedia"

	if ps.signer == nil {
		for i := range items {
			items[i].Media = make([]dto.PostMedia, 0)
		}
		return nil
	}

	indexes := make([]string, len(items))

	for i := range items {
		indexes[i] = items[i].ID
	}

	rows, err := ps.storage.LoadMedia(ctx, indexes)
	if err != nil {
		return fmt.Errorf("%s:%w", op, err)
	}

	for i := range items {
		list := rows[items[i].ID]
		items[i].Media = make([]dto.PostMedia, 0, len(list))
		for _, row := range list {
			m, ok, err := ps.toPostMedia(ctx, row)
			if err != nil {
				return fmt.Errorf("%s:%w", op, err)
			}
			if ok {
				items[i].Media = append(items[i].Media, m)
			}
		}
	}
	return nil
}

func (ps *PostService) toPostMedia(ctx context.Context, row dto.PostMediaRow) (dto.PostMedia, bool, error) {
	if row.StorageKey == nil || row.MediaType == nil {
		return dto.PostMedia{}, false, nil
	}

	bucket := ps.signer.PrivateBucket()

	url, _, err := ps.signer.PresignGet(ctx, bucket, *row.StorageKey)
	if err != nil {
		return dto.PostMedia{}, false, err
	}

	var thumb *string

	if row.ThumbKey != nil && *row.ThumbKey != "" {
		prethumb, _, err := ps.signer.PresignGet(ctx, bucket, *row.ThumbKey)
		if err != nil {
			return dto.PostMedia{}, false, err
		}
		thumb = &prethumb
	}
	return dto.PostMedia{
		UploadID: row.UploadID,
		File: media.File{
			MediaType: *row.MediaType,
			URL:       url,
			ThumbURL:  thumb,
			Blurhash:  row.Blurhash,
			Width:     row.Width,
			Height:    row.Height,
		},
	}, true, nil
}

// authorAvatar builds the author's avatar from the joined upload columns (the thumb is enough for posts).
func (ps *PostService) authorAvatar(e dto.PostItem) *media.File {
	return media.Build(ps.urls, media.Row{
		MediaType:  e.AuthorAvatarType,
		StorageKey: e.AuthorAvatarKey,
		ThumbKey:   e.AuthorAvatarThumb,
		Blurhash:   e.AuthorAvatarBlurhash,
	})
}

func (ps *PostService) CreatePost(ctx context.Context, authorID string, req dto.CreatePostRequest) (dto.PostItem, error) {
	const op = "internal/features/post/service CreatePost"

	log := ps.log.With(
		zap.String("op", op),
		zap.String("user_id", authorID),
	)

	parsedEmbeds, err := parseEmbeds(req.Embeds)
	if err != nil {
		return dto.PostItem{}, err
	}

	item, err := ps.storage.CreatePost(ctx, authorID, req.Title, req.Body, req.Media, parsedEmbeds)
	if err != nil {
		log.Error("error", zap.Error(err))
		return dto.PostItem{}, fmt.Errorf("%s:%w", op, err)
	}

	onePost := []dto.PostItem{item}

	if err := ps.attachEmbeds(ctx, onePost); err != nil {
		log.Error("error while tried to attach embed to single post", zap.Error(err))
		return dto.PostItem{}, fmt.Errorf("%s:%w", op, err)
	}

	if err := ps.getPostsWithMedia(ctx, onePost); err != nil {
		log.Error("error while tried to sign post with media", zap.Error(err))
		return dto.PostItem{}, fmt.Errorf("%s:%w", op, err)
	}

	item = onePost[0]

	item.AuthorAvatar = ps.authorAvatar(item)
	return item, nil
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

	able, err := ps.storage.CanReadPostsOf(ctx, callerID, post.AuthorUsername)
	if err != nil {
		return dto.PostItem{}, fmt.Errorf("%s:%w", op, err)
	}
	if !able {
		return dto.PostItem{}, sharederrors.ErrPostAccessDenied
	}

	onePost := []dto.PostItem{post}

	if err := ps.attachEmbeds(ctx, onePost); err != nil {
		log.Error("error while tried to attach embed to single post", zap.Error(err))
		return dto.PostItem{}, fmt.Errorf("%s:%w", op, err)
	}

	if err := ps.getPostsWithMedia(ctx, onePost); err != nil {
		log.Error("error while tried to sign post with media", zap.Error(err))
		return dto.PostItem{}, fmt.Errorf("%s:%w", op, err)
	}

	post = onePost[0]

	post.AuthorAvatar = ps.authorAvatar(post)
	return post, err
}

// UpdatePost applies the changes and returns the updated post (with fresh media URLs).
func (ps *PostService) UpdatePost(ctx context.Context, authorID, postID string, req dto.UpdatePostRequest) (dto.PostItem, error) {
	const op = "internal/features/post/service UpdatePost"

	log := ps.log.With(
		zap.String("op", op),
		zap.String("user_id", authorID),
		zap.String("post_id", postID),
	)

	var embeds *[]vidembed.Embed

	if req.Embeds != nil {
		parsedEmbeds, err := parseEmbeds(*req.Embeds)
		if err != nil {
			return dto.PostItem{}, err
		}
		embeds = &parsedEmbeds
	}

	previous, err := ps.storage.UpdatePost(ctx, postID, authorID, req.Title, req.Body, req.Media, embeds)
	if err != nil {
		if errors.Is(err, sharederrors.ErrPostNotFound) {
			return dto.PostItem{}, sharederrors.ErrPostNotFound
		}
		if errors.Is(err, sharederrors.ErrInvalidMedia) {
			return dto.PostItem{}, sharederrors.ErrInvalidMedia
		}
		log.Error("unable to update post", zap.Error(err))
		return dto.PostItem{}, fmt.Errorf("%s:%w", op, err)
	}

	if req.Media != nil {
		ps.deleteUploads(ctx, log, authorID, detached(previous, *req.Media))
	}

	// read it back: the caller is the author, so no access check is needed here
	post, err := ps.storage.GetPostByID(ctx, postID)
	if err != nil {
		log.Error("unable to read the updated post", zap.Error(err))
		return dto.PostItem{}, fmt.Errorf("%s:%w", op, err)
	}

	onePost := []dto.PostItem{post}

	if err := ps.attachEmbeds(ctx, onePost); err != nil {
		log.Error("error while tried to attach embed to single post", zap.Error(err))
		return dto.PostItem{}, fmt.Errorf("%s:%w", op, err)
	}

	if err := ps.getPostsWithMedia(ctx, onePost); err != nil {
		log.Error("unable to load media of the updated post", zap.Error(err))
		return dto.PostItem{}, fmt.Errorf("%s:%w", op, err)
	}
	post = onePost[0]

	post.AuthorAvatar = ps.authorAvatar(post)
	return post, nil
}

func (ps *PostService) DeletePost(ctx context.Context, authorID, postID string) error {
	const op = "internal/features/post/service DeletePost"
	// TODO

	log := ps.log.With(
		zap.String("op", op),
		zap.String("user_id", authorID),
	)

	uploadIDs, err := ps.storage.DeletePost(ctx, postID, authorID)
	if err != nil {
		if errors.Is(err, sharederrors.ErrPostNotFound) {
			return sharederrors.ErrPostNotFound
		}
		log.Error("unable to delete post", zap.Error(err))

		return fmt.Errorf("%s:%w", op, err)
	}

	ps.deleteUploads(ctx, log, authorID, uploadIDs)

	return nil
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
			AuthorURL:      httpfeatures.BuildUrl(ps.baseurl, e.AuthorUsername),
			AuthorAvatar:   ps.authorAvatar(e),
			Title:          e.Title,
			Body:           e.Body,
			CreatedAt:      e.CreatedAt,
			UpdatedAt:      e.UpdatedAt,
		})
	}

	if err := ps.attachEmbeds(ctx, items); err != nil {
		log.Error("error while tried to attach embed to posts", zap.Error(err))
		return nil, fmt.Errorf("%s:%w", op, err)
	}

	if err := ps.getPostsWithMedia(ctx, items); err != nil {
		log.Error("error while tried to sign post with media", zap.Error(err))
		return nil, fmt.Errorf("%s:%w", op, err)
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
		entities, err = ps.storage.GetPostsByAuthorID(ctx, callerID, cursorTime, cursorID, params.Limit+1)
		if err != nil {
			log.Error("err", zap.Error(err))

			return nil, fmt.Errorf("%s:%w", op, err)
		}
	default:
		entities, err = ps.storage.GetPostsByAuthorUsername(ctx, username, cursorTime, cursorID, params.Limit+1)
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
			AuthorURL:      httpfeatures.BuildUrl(ps.baseurl, e.AuthorUsername),
			AuthorAvatar:   ps.authorAvatar(e), //add url builder by username
			Title:          e.Title,
			Body:           e.Body,
			CreatedAt:      e.CreatedAt,
			UpdatedAt:      e.UpdatedAt,
		})
	}

	if err := ps.attachEmbeds(ctx, items); err != nil {
		log.Error("error while tried to attach embed to posts", zap.Error(err))
		return nil, fmt.Errorf("%s:%w", op, err)
	}

	if err := ps.getPostsWithMedia(ctx, items); err != nil {
		log.Error("error while tried to sign post with media", zap.Error(err))
		return nil, fmt.Errorf("%s:%w", op, err)
	}

	return &dto.PostListResponse{
		Items:      items,
		NextCursor: nextCursor,
		HasMore:    hasMore,
		Message:    "",
	}, err
}
