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
		WinnersQuantity int        `json:"winners_quantity"`
		StartFrom       *time.Time `json:"winners_became_referrals_since"`
		EndUntil        *time.Time `json:"referrals_can_be_winners_until"`
	}
	ContestBetweenReferralsResponse struct {
		Winners []WinnersDTO `json:"winners"`
		Message string       `json:"message"`
	}

	WinnersDTO struct {
		Place         int       `json:"place"`
		Username      string    `json:"winner_username" db:"username"`
		UserURL       string    `json:"winner_user_url" `
		ReferralSince time.Time `json:"winner_referral_since" db:"referral_timestamp"`
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
