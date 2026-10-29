package dto

import "time"

type (
	ChangeUserPasswordRequest struct {
		OldPass string
		NewPass string
	}
	ChangeUserPasswordResponse struct {
		Message string
	}
	ChangeReferrerRequest struct {
		ReferralCode string
	}
	ChangeReferrerResponse struct {
		ReferralCooldown string
		Message          string
	}

	CreateReferralKeyRequest struct {
		ReferralKey string `json:"referral_key"`
	}

	CreateReferralKeyResponse struct {
		Message string `json:"message"`
	}

	DeleteAccountRequest struct {
		Password string
	}
	DeleteAccountResponse struct {
		Message string
	}

	UserItems struct {
		Username        string     `json:"username"`
		OwnReferral     *string    `json:"own_referral_key,omitempty"`
		ReferredBy      *string    `json:"referred_by_username,omitempty"`
		LastTimeRefUsed *time.Time `json:"last_time_ref_used,omitempty"`
		IsBlocked       bool       `json:"is_blocked"`
		CreatedAt       time.Time  `json:"created_at"`
	}

	UserByUsernameResponse struct {
		User    UserItems `json:"user_items"`
		Message string    `json:"message"`
	}

	SelfUserItems struct {
		Username        string     `json:"username"`
		ConnectedEmail  string     `json:"connected_email"`
		OwnReferral     *string    `json:"own_referral_key,omitempty"`
		ReferralCount   int        `json:"referral_count"`
		ReferredBy      *string    `json:"referred_by_username,omitempty"`
		LastTimeRefUsed *time.Time `json:"last_time_ref_used,omitempty"`
		IsBlocked       bool       `json:"is_blocked"`
		CreatedAt       time.Time  `json:"created_at"`
		LastLogin       time.Time  `json:"last_login,omitempty"`
	}
	SelfUser struct {
		User    SelfUserItems `json:"user"`
		Message string        `json:"message"`
	}
)
