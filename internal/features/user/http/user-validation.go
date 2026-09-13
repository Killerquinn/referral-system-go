package http

import (
	"errors"
	"regexp"
	"unicode/utf8"

	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/killerquinn/referral-system-go/internal/features/user/dto"
)

var (
	reHasDigit  = regexp.MustCompile(`[0-9]`)
	reHasLetter = regexp.MustCompile(`[a-zA-Z]`)
)

func ValidateChangePassword(req *dto.ChangeUserPasswordRequest) error {
	return validation.ValidateStruct(&req,
		validation.Field(&req.NewPass, validation.Required, validation.By(validatePassword)),
		validation.Field(&req.OldPass, validation.Required),
	)
}

func validatePassword(value interface{}) error {
	s, ok := value.(string)
	if !ok || s == "" {
		return nil
	}

	length := utf8.RuneCountInString(s)
	if length < 8 {
		return errors.New("password must be at least 8 characters long")
	}
	if len(s) > 72 {
		return errors.New("password is too long (max 72 bytes)")
	}

	if !reHasDigit.MatchString(s) {
		return errors.New("password must contain at least one digit")
	}

	if !reHasLetter.MatchString(s) {
		return errors.New("password must contain at least one letter")
	}

	return nil
}
