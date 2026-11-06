package http

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/killerquinn/referral-system-go/internal/features/post/dto"
	httpfeatures "github.com/killerquinn/referral-system-go/internal/infrastructure/pkg/http-features"
	"github.com/killerquinn/referral-system-go/internal/infrastructure/pkg/jwtcontext"
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

	r.Post("/api/v1/posts", newPostHandler.CreatePost)
	r.Get("/api/v1/posts/feed", newPostHandler.Feed)
	r.Get("/api/v1/posts/user", newPostHandler.PostsOfUser)              // own posts
	r.Get("/api/v1/posts/user/{identifier}", newPostHandler.PostsOfUser) // posts of a user
	r.Get("/api/v1/posts/{postID}", newPostHandler.GetPost)
	r.Put("/api/v1/posts/{postID}", newPostHandler.UpdatePost)
	r.Delete("/api/v1/posts/{postID}", newPostHandler.DeletePost)
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
	// TODO: chi.URLParam(r, "postID") must be a uuid
	responseReturn(w, http.StatusNotImplemented, dto.GetPostResponse{Message: "not implemented"})
}

func (ph *PostHandler) UpdatePost(w http.ResponseWriter, r *http.Request) {
	const op = "internal/features/post/http UpdatePost"
	// TODO
	responseReturn(w, http.StatusNotImplemented, dto.UpdatePostResponse{Message: "not implemented"})
}

func (ph *PostHandler) DeletePost(w http.ResponseWriter, r *http.Request) {
	const op = "internal/features/post/http DeletePost"
	// TODO
	responseReturn(w, http.StatusNotImplemented, dto.DeletePostResponse{Message: "not implemented"})
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
