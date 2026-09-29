package sharederrors

import "errors"

var (
	ErrInvalidCreds                      = errors.New("Error invalid credentials")
	ErrUserNotFound                      = errors.New("Error user not found")
	ErrUserAlreadyRegistered             = errors.New("User already exist")
	ErrSessionNotFound                   = errors.New("Session not found")
	ErrInvalidUUID                       = errors.New("Unable to parse string into UUID")
	ErrUnnableToChangePassword           = errors.New("Unable to change password")
	ErrReferrerOrReferralCodeDoesntExist = errors.New("Unable to change referrer, code or user doesnt exist")
	ErrCooldownNotPassedYet              = errors.New("Error: cooldown not passed yet")
	ErrSelfReferred                      = errors.New("Error, self-referring is forbidden")
	ErrUserIsNotReferredYet              = errors.New("Unable to check whose user referral, isn't referred yet")
	ErrUserIsBanned                      = errors.New("Error user is banned. Any interactions with this user unavaible now")
	ErrUserDoesntHaveReferralsYet        = errors.New("User doesnt have referrals yet")
	ErrInvalidCursorFormat               = errors.New("Error client gave invalid cursor format: unnable to show next pages of referrals")

// ErrInsufficientQuantityOfReferrals   = errors.New("User have insufficient quantity of referrals to start contest with this much of winners")
)
