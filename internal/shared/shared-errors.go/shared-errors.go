package sharederrors

import "errors"

var (
	ErrInvalidCreds          = errors.New("Error invalid credentials")
	ErrUserNotFound          = errors.New("Error user not found")
	ErrUserAlreadyRegistered = errors.New("User already exist")
)
