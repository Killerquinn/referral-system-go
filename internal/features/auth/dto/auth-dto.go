package dto

type (
	RegisterNewUserRequest struct {
		Username string
		Email    string
		Password string
	}
	LoginUserRequest struct {
		Email    string
		Password string
	}
	LogoutUserRequest struct {
		UserID string
	}
	Tokens struct {
		Access  string `json:"access"`
		Refresh string `json:"refresh"`
	}
	SessionMetadata struct {
		UserAgent string
		ClientIP  string
	}

	RefreshRequest struct {
		Refresh string
	}
)
