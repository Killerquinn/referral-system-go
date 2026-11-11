package dto

import "time"

type (
	// ===== entity =====

	PostItem struct {
		ID             string     `json:"id" db:"id"`
		AuthorID       string     `json:"-" db:"author_id"`
		AuthorUsername string     `json:"author_username" db:"author_username"`
		AuthorURL      string     `json:"author_url"`
		Title          string     `json:"title" db:"title"`
		Body           string     `json:"body" db:"body"`
		CreatedAt      time.Time  `json:"created_at" db:"created_at"`
		UpdatedAt      *time.Time `json:"updated_at,omitempty" db:"updated_at"`
	}

	// ===== create =====

	CreatePostRequest struct {
		Title string `json:"title"`
		Body  string `json:"body"`
	}
	CreatePostResponse struct {
		Post    *PostItem `json:"post,omitempty"`
		Message string    `json:"message"`
	}

	// ===== get one =====

	GetPostResponse struct {
		Post    *PostItem `json:"post,omitempty"`
		Message string    `json:"message"`
	}

	// ===== update =====

	// nil field = leave as is
	UpdatePostRequest struct {
		Title *string `json:"title,omitempty"`
		Body  *string `json:"body,omitempty"`
	}
	UpdatePostResponse struct {
		Message string `json:"message"`
	}

	// ===== delete =====

	DeletePostResponse struct {
		Message string `json:"message"`
	}

	// ===== lists (feed / posts of a user), cursor based =====

	PostListParams struct {
		Limit  int    `json:"limit"`
		Cursor string `json:"cursor"` // base64(RawURL) of "created_at|id" of the last item on the previous page
	}

	PostListResponse struct {
		Items      []PostItem `json:"items"`
		NextCursor *string    `json:"next_cursor,omitempty"`
		HasMore    bool       `json:"has_more"`
		Message    string     `json:"message"`
	}
)
