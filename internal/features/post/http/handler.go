package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
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
	UpdatePost(ctx context.Context, authorID, postID string, req dto.UpdatePostRequest) (dto.PostItem, error)
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
	}

	postItem, err := ph.pService.GetPost(r.Context(), userID, trimmedPostID)
	if err != nil {
		if errors.Is(err, sharederrors.ErrPostNotFound) {
			responseReturn(w, http.StatusNotFound, dto.GetPostResponse{})
			return
		}
		if errors.Is(err, sharederrors.ErrPostAccessDenied) {
			responseReturn(w, http.StatusNotAcceptable, dto.GetPostResponse{})
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

	updatedPost, err := ph.pService.UpdatePost(r.Context(), userID, trimmedPostID, req)
	if err != nil {
		responseReturn(w, http.StatusInternalServerError, dto.UpdatePostResponse{})
		return
	}

	responseReturn(w, http.StatusOK, dto.UpdatePostResponse{Post: &updatedPost, Message: "post successfully updated!"})
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
	responseReturn(w, http.StatusNotImplemented, dto.PostListResponse{Items: []dto.PostItem{}, Message: "not implemented"})
}

func (ph *PostHandler) PostsOfUser(w http.ResponseWriter, r *http.Request) {
	const op = "internal/features/post/http PostsOfUser"
	// TODO: identifier optional, validate with httpfeatures.ValidUsernameLength
	responseReturn(w, http.StatusNotImplemented, dto.PostListResponse{Items: []dto.PostItem{}, Message: "not implemented"})
}

func responseReturn(w http.ResponseWriter, status int, resp any, headers ...httpfeatures.Header) {
	w.Header().Set("Content-Type", "application/json")

	for _, h := range headers {
		w.Header().Set(h.Key, h.Value)
	}

	w.WriteHeader(status)
	json.NewEncoder(w).Encode(resp)
}
