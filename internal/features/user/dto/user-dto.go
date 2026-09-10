package dto

type (
	ChangeUserPasswordRequest struct {
		UserID  string
		OldPass string
		NewPass string
	}
)
