package dto

import "time"

type (
	SeeWhoseReferralAlreadyRequest struct {
		Username string `json:"username"`
	}
	SeeWhoseReferralAlreadyResponse struct {
		Referrer      string    `json:"referrer"`
		ReferrerURL   string    `json:"referrer_url"`
		ReferralSince time.Time `json:"referral_since"`
		Message       string    `json:"message"`
	}
	ContestBetweenReferralsRequest struct {
	}
	ContestBetweenReferralsResponse struct {
	}
	CurrentReferralListRequest struct {
		Identifier string `json:"indentifier"`
		Limit      int    `json:"limit"`
		Cursor     string `json:"cursor"`
	}

	ReferralListParams struct {
		Limit  int    `json:"limit"`
		Cursor string `json:"cursor"` // Date of last item at previous page
	}

	ReferralItem struct {
		ID         string    `json:"id"`
		Username   string    `json:"username"`
		ProfileURL string    `json:"profile_url"`
		JoinedAt   time.Time `json:"joined_at"`
	}

	CurrentReferralListResponse struct {
		Items      []ReferralItem `json:"items"`
		NextCursor *string        `json:"next_cursor,omitempty"` // nil if it have no more elements || To-Do: make it in base64 in format rawCursor := fmt.Sprintf("%s|%s", lastItem.CreatedAt.Format(time.RFC3339Nano), lastItem.ID)
		HasMore    bool           `json:"has_more"`
		Message    string         `json:"message"`
	}
)
