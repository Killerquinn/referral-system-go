package dto

type (
	ChangeUserPasswordRequest struct {
		UserID  string
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
)
