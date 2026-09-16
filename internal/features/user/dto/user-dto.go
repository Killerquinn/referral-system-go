package dto

import "time"

type (
	ChangeUserPasswordRequest struct {
		UserID  string
		OldPass string
		NewPass string
	}
	ChangeReferrerRequest struct {
		ReferralCode string
	}
	ChangeReferrerResponse struct {
		ReferralCooldown time.Time
	}
)
