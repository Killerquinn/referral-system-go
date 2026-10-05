package dto

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
		ReferralKey string
	}

	CreateReferralKeyResponse struct {
		Message string
	}

	DeleteAccountRequest struct {
		Password string
	}
	DeleteAccountResponse struct {
		Message string
	}
)
