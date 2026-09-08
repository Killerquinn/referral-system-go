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
		Access  string
		Refresh string
	}
	SessionMetadata struct {
		UserAgent string
		ClientIP  string
	}
)
