package sharederrors

import "errors"

var (
	ErrInvalidCreds                      = errors.New("Error invalid credentials")
	ErrUserNotFound                      = errors.New("Error user not found")
	ErrUserAlreadyRegistered             = errors.New("User already exist")
	ErrSessionNotFound                   = errors.New("Session not found")
	ErrInvalidUUID                       = errors.New("Unnable to parse string into UUID")
	ErrUnnableToChangePassword           = errors.New("Unnable to change password")
	ErrReferrerOrReferralCodeDoesntExist = errors.New("Unnable to change referrer, code or user doesnt exist")
	ErrCooldownNotPassedYet              = errors.New("Error: cooldown not passed yet")
	ErrSelfReferred                      = errors.New("Error, self-referring is forbidden")
)
