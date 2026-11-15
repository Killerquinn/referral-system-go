package dto

import (
	"time"

	"github.com/killerquinn/referral-system-go/internal/shared/media"
)

type (
	ChangeUserPasswordRequest struct {
		OldPass string `json:"old_password"`
		NewPass string `json:"new_password"`
	}
	ChangeUserPasswordResponse struct {
		Message string `json:"message"`
	}
	ChangeReferrerRequest struct {
		ReferralCode string `json:"referral_code"`
	}
	ChangeReferrerResponse struct {
		ReferralCooldown string `json:"referral_cooldown"`
		Message          string `json:"message"`
	}

	CreateReferralKeyRequest struct {
		ReferralKey string `json:"referral_key"`
	}

	CreateReferralKeyResponse struct {
		Message string `json:"message"`
	}

	DeleteAccountRequest struct {
		Password string `json:"password"`
	}
	DeleteAccountResponse struct {
		Message string `json:"message"`
	}

	UserItems struct {
		Username        string      `json:"username"`
		OwnReferral     *string     `json:"own_referral_key,omitempty"`
		ReferredBy      *string     `json:"referred_by_username,omitempty"`
		LastTimeRefUsed *time.Time  `json:"last_time_ref_used,omitempty"`
		IsBlocked       bool        `json:"is_blocked"`
		CreatedAt       time.Time   `json:"created_at"`
		Avatar          *media.File `json:"avatar,omitempty"`
		AvatarRow       media.Row   `json:"-"` // filled by the repo, turned into Avatar by the service
	}

	UserByUsernameResponse struct {
		User    UserItems `json:"user_items"`
		Message string    `json:"message"`
	}

	SelfUserItems struct {
		Username        string      `json:"username"`
		ConnectedEmail  string      `json:"connected_email"`
		OwnReferral     *string     `json:"own_referral_key,omitempty"`
		ReferralCount   int         `json:"referral_count"`
		ReferredBy      *string     `json:"referred_by_username,omitempty"`
		LastTimeRefUsed *time.Time  `json:"last_time_ref_used,omitempty"`
		IsBlocked       bool        `json:"is_blocked"`
		CreatedAt       time.Time   `json:"created_at"`
		LastLogin       time.Time   `json:"last_login,omitempty"`
		Avatar          *media.File `json:"avatar,omitempty"`
		AvatarRow       media.Row   `json:"-"`
	}

	// upload_id: an uploaded, ready file with purpose "avatar"; null removes the avatar
	SetAvatarRequest struct {
		UploadID *string `json:"upload_id"`
	}

	SetAvatarResponse struct {
		Avatar  *media.File `json:"avatar,omitempty"`
		Message string      `json:"message"`
	}
	SelfUser struct {
		User    SelfUserItems `json:"user"`
		Message string        `json:"message"`
	}
)
