package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"uuid"

	"github.com/go-chi/chi/v5"
	"github.com/killerquinn/referral-system-go/internal/features/post/dto"
	httpfeatures "github.com/killerquinn/referral-system-go/internal/infrastructure/pkg/http-features"
	"github.com/killerquinn/referral-system-go/internal/infrastructure/pkg/jwtcontext"
	sharederrors "github.com/killerquinn/referral-system-go/internal/shared/shared-errors.go"
)

type PostService interface {
	CreatePost(ctx context.Context, authorID string, req dto.CreatePostRequest) (dto.PostItem, error)
	GetPost(ctx context.Context, callerID, postID string) (dto.PostItem, error)
	UpdatePost(ctx context.Context, authorID, postID string, req dto.UpdatePostRequest) error
	DeletePost(ctx context.Context, authorID, postID string) error
	Feed(ctx context.Context, callerID string, params dto.PostListParams) (*dto.PostListResponse, error)
	PostsOfUser(ctx context.Context, callerID, username string, params dto.PostListParams) (*dto.PostListResponse, error)
}

type PostHandler struct {
	pService PostService
}

func NewPostHandler(ps PostService) *PostHandler {
	return &PostHandler{pService: ps}
}

/*
Routes (add inside the AuthMiddleware group in rest/chi.go):

	r.Post("/api/v1/posts", newPostHandler.CreatePost) done
	r.Get("/api/v1/posts/feed", newPostHandler.Feed)
	r.Get("/api/v1/posts/user", newPostHandler.PostsOfUser)              // own posts
	r.Get("/api/v1/posts/user/{identifier}", newPostHandler.PostsOfUser) // posts of a user
	r.Get("/api/v1/posts/{postID}", newPostHandler.GetPost) done
	r.Put("/api/v1/posts/{postID}", newPostHandler.UpdatePost) done
	r.Delete("/api/v1/posts/{postID}", newPostHandler.DeletePost) done
*/

const (
	defaultLimit   = 20
	maxLimit       = 100
	maxTitleLength = 200
	maxBodyLength  = 10000
)

func (ph *PostHandler) CreatePost(w http.ResponseWriter, r *http.Request) {
	const op = "internal/features/post/http CreatePost"

	userID, ok := r.Context().Value(jwtcontext.UserIDKey).(string)
	if !ok {
		responseReturn(w, http.StatusUnauthorized, dto.CreatePostResponse{})
		return
	}

	var req dto.CreatePostRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responseReturn(w, http.StatusBadRequest, dto.CreatePostResponse{})
		return
	}

	if len(req.Body) > maxBodyLength {
		responseReturn(w, http.StatusBadRequest, dto.CreatePostResponse{})
		return
	}

	item, err := ph.pService.CreatePost(r.Context(), userID, req)
	if err != nil {
		responseReturn(w, http.StatusInternalServerError, dto.CreatePostResponse{})
		return
	}

	responseReturn(w, http.StatusOK, dto.CreatePostResponse{Post: &item, Message: "Post successfully created"})
}

func (ph *PostHandler) GetPost(w http.ResponseWriter, r *http.Request) {
	const op = "internal/features/post/http GetPost"
	userID, ok := r.Context().Value(jwtcontext.UserIDKey).(string)
	if !ok {
		responseReturn(w, http.StatusUnauthorized, dto.GetPostResponse{})
		return
	}

	postID := chi.URLParam(r, "postID")

	trimmedPostID := strings.TrimSpace(postID)
	if len(trimmedPostID) == 0 {
		responseReturn(w, http.StatusBadRequest, dto.GetPostResponse{Message: "post id cannot be empty"})
		return
	}
	_, err := uuid.Parse(trimmedPostID)
	if err != nil {
		responseReturn(w, http.StatusBadRequest, dto.GetPostResponse{Message: "invalid post id format"})
		return
	}

	postItem, err := ph.pService.GetPost(r.Context(), userID, trimmedPostID)
	if err != nil {
		if errors.Is(err, sharederrors.ErrPostNotFound) {
			responseReturn(w, http.StatusNotFound, dto.GetPostResponse{Message: "post not found"})
			return
		}
		if errors.Is(err, sharederrors.ErrPostAccessDenied) {
			responseReturn(w, http.StatusForbidden, dto.GetPostResponse{Message: "not allowed"})
			return
		}
		if errors.Is(err, sharederrors.ErrUserIsBanned) {
			responseReturn(w, http.StatusForbidden, dto.GetPostResponse{Message: "author is banned"})
			return
		}
		responseReturn(w, http.StatusInternalServerError, dto.GetPostResponse{})
		return
	}

	responseReturn(w, http.StatusOK, dto.GetPostResponse{Post: &postItem, Message: "post successfully found"})
}

func (ph *PostHandler) UpdatePost(w http.ResponseWriter, r *http.Request) {
	const op = "internal/features/post/http UpdatePost"

	userID, ok := r.Context().Value(jwtcontext.UserIDKey).(string)
	if !ok {
		responseReturn(w, http.StatusUnauthorized, dto.UpdatePostResponse{})
		return
	}

	postID := chi.URLParam(r, "postID")
	trimmedPostID := strings.TrimSpace(postID)
	if len(trimmedPostID) == 0 {
		responseReturn(w, http.StatusBadRequest, dto.UpdatePostResponse{Message: "post id cannot be empty"})
		return
	}
	_, err := uuid.Parse(trimmedPostID)
	if err != nil {
		responseReturn(w, http.StatusBadRequest, dto.UpdatePostResponse{Message: "invalid post id format"})
		return
	}

	var req dto.UpdatePostRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responseReturn(w, http.StatusBadRequest, dto.UpdatePostResponse{})
		return
	}
	if req.Body != nil {
		if len(*req.Body) > maxBodyLength {
			responseReturn(w, http.StatusBadRequest, dto.UpdatePostResponse{})
			return
		}
	}

	err = ph.pService.UpdatePost(r.Context(), userID, trimmedPostID, req)
	if err != nil {
		if errors.Is(err, sharederrors.ErrPostNotFound) {
			responseReturn(w, http.StatusNotFound, dto.UpdatePostResponse{Message: "post not found"})
			return
		}
		responseReturn(w, http.StatusInternalServerError, dto.UpdatePostResponse{})
		return
	}

	responseReturn(w, http.StatusOK, dto.UpdatePostResponse{Message: "post successfully updated!"})
}

func (ph *PostHandler) DeletePost(w http.ResponseWriter, r *http.Request) {
	const op = "internal/features/post/http DeletePost"

	userID, ok := r.Context().Value(jwtcontext.UserIDKey).(string)
	if !ok {
		responseReturn(w, http.StatusUnauthorized, dto.DeletePostResponse{})
		return
	}

	postID := chi.URLParam(r, "postID")
	trimmedPostID := strings.TrimSpace(postID)
	if len(trimmedPostID) == 0 {
		responseReturn(w, http.StatusBadRequest, dto.DeletePostResponse{Message: "post id cannot be empty"})
		return
	}
	_, err := uuid.Parse(trimmedPostID)
	if err != nil {
		responseReturn(w, http.StatusBadRequest, dto.DeletePostResponse{Message: "invalid post id format"})
		return
	}

	if err := ph.pService.DeletePost(r.Context(), userID, trimmedPostID); err != nil {
		if errors.Is(err, sharederrors.ErrPostNotFound) {
			responseReturn(w, http.StatusNotFound, dto.DeletePostResponse{Message: "post not found"})
			return
		}
		responseReturn(w, http.StatusInternalServerError, dto.DeletePostResponse{})
		return
	}

	responseReturn(w, http.StatusOK, dto.DeletePostResponse{Message: "post successfully deleted"})
}

func (ph *PostHandler) Feed(w http.ResponseWriter, r *http.Request) {
	const op = "internal/features/post/http Feed"
	// TODO: ?cursor=&limit=

	userID, ok := r.Context().Value(jwtcontext.UserIDKey).(string)
	if !ok {
		responseReturn(w, http.StatusUnauthorized, dto.PostListResponse{Items: []dto.PostItem{}, Message: "unauthorized"})
		return
	}

	cursor := r.URL.Query().Get("cursor")
	limitStr := r.URL.Query().Get("limit")

	parsedLimit := parseLimit(limitStr)

	params := dto.PostListParams{
		Cursor: cursor,
		Limit:  parsedLimit,
	}

	resp, err := ph.pService.Feed(r.Context(), userID, params)
	if err != nil {
		if errors.Is(err, sharederrors.ErrUserIsNotReferredYet) {
			responseReturn(w, http.StatusNotFound, dto.PostListResponse{Items: []dto.PostItem{}, Message: "you are not referred yet"})
			return
		}
		if errors.Is(err, sharederrors.ErrInvalidCursorFormat) {
			responseReturn(w, http.StatusBadRequest, dto.PostListResponse{Items: []dto.PostItem{}, Message: "invalid cursor format"})
			return
		}
		if errors.Is(err, sharederrors.ErrInvalidCursorTime) {
			responseReturn(w, http.StatusBadRequest, dto.PostListResponse{Items: []dto.PostItem{}, Message: "invalid time of cursor"})
			return
		}
		if errors.Is(err, sharederrors.ErrUserIsBanned) {
			responseReturn(w, http.StatusForbidden, dto.PostListResponse{Items: []dto.PostItem{}, Message: "referrer is banned"})
			return
		}
		responseReturn(w, http.StatusInternalServerError, dto.PostListResponse{Items: []dto.PostItem{}, Message: "internal server error"})
		return
	}

	responseReturn(w, http.StatusOK, resp)
}

func (ph *PostHandler) PostsOfUser(w http.ResponseWriter, r *http.Request) {
	const op = "internal/features/post/http PostsOfUser"
	// TODO: identifier optional, validate with httpfeatures.ValidUsernameLength
	var targetID string

	userID, ok := r.Context().Value(jwtcontext.UserIDKey).(string) //use it in case if identifier is empty
	if !ok {
		responseReturn(w, http.StatusUnauthorized, dto.PostListResponse{Items: []dto.PostItem{}, NextCursor: nil, HasMore: false, Message: "unauthorized"})
		return
	}

	identifier := chi.URLParam(r, "identifier")
	cursor := r.URL.Query().Get("cursor")
	limitStr := r.URL.Query().Get("limit")

	parsedLimit := parseLimit(limitStr)

	preparedIdentifier := strings.TrimSpace(identifier)

	if preparedIdentifier == "" {
		targetID = userID
	} else {
		_, err := uuid.Parse(preparedIdentifier)
		if err == nil {
			targetID = userID //not allowing user to find someone by userID
		} else {
			if !httpfeatures.ValidUsernameLength(preparedIdentifier) {
				responseReturn(w, http.StatusBadRequest, dto.PostListResponse{Items: []dto.PostItem{}, NextCursor: nil, HasMore: false, Message: fmt.Sprintf("username must be %d-%d characters long", httpfeatures.UsernameMinLen, httpfeatures.UsernameMaxLen)})
				return
			}
			targetID = preparedIdentifier
		}
	}

	params := dto.PostListParams{
		Cursor: cursor,
		Limit:  parsedLimit,
	}

	resp, err := ph.pService.PostsOfUser(r.Context(), userID, targetID, params)
	if err != nil {
		if errors.Is(err, sharederrors.ErrPostAccessDenied) {
			responseReturn(w, http.StatusForbidden, dto.PostListResponse{Items: []dto.PostItem{}, NextCursor: nil, HasMore: false, Message: "not allowed"})
			return
		}
		if errors.Is(err, sharederrors.ErrInvalidCursorFormat) {
			responseReturn(w, http.StatusBadRequest, dto.PostListResponse{Items: []dto.PostItem{}, NextCursor: nil, HasMore: false, Message: "invalid cursor format"})
			return
		}
		if errors.Is(err, sharederrors.ErrInvalidCursorTime) {
			responseReturn(w, http.StatusBadRequest, dto.PostListResponse{Items: []dto.PostItem{}, NextCursor: nil, HasMore: false, Message: "invalid time of cursor"})
			return
		}
		if errors.Is(err, sharederrors.ErrUserIsBanned) {
			responseReturn(w, http.StatusForbidden, dto.PostListResponse{Items: []dto.PostItem{}, NextCursor: nil, HasMore: false, Message: "user is banned"})
			return
		}
		if errors.Is(err, sharederrors.ErrUserNotFound) {
			responseReturn(w, http.StatusNotFound, dto.PostListResponse{Items: []dto.PostItem{}, NextCursor: nil, HasMore: false, Message: "user not found"})
			return
		}
		responseReturn(w, http.StatusInternalServerError, dto.PostListResponse{Items: []dto.PostItem{}, NextCursor: nil, HasMore: false, Message: "internal server error"})
		return
	}

	responseReturn(w, http.StatusOK, resp)
}

func responseReturn(w http.ResponseWriter, status int, resp any, headers ...httpfeatures.Header) {
	w.Header().Set("Content-Type", "application/json")

	for _, h := range headers {
		w.Header().Set(h.Key, h.Value)
	}

	w.WriteHeader(status)
	json.NewEncoder(w).Encode(resp)
}

func parseLimit(limitStr string) int {
	if limitStr == "" {
		return defaultLimit
	}

	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		return defaultLimit
	}

	if limit <= 0 {
		return defaultLimit
	}

	if limit > maxLimit {
		return maxLimit
	}

	return limit
}
