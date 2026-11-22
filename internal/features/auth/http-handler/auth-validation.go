package httphandler

import (
	"errors"
	"fmt"
	"net/mail"
	"regexp"
	"unicode/utf8"

	ozzoval "github.com/go-ozzo/ozzo-validation"
	"github.com/killerquinn/referral-system-go/internal/features/auth/dto"
	httpfeatures "github.com/killerquinn/referral-system-go/internal/infrastructure/pkg/http-features"
)

var (
	reHasDigit  = regexp.MustCompile(`[0-9]`)
	reHasLetter = regexp.MustCompile(`[a-zA-Z]`)
	// emailed codes are exactly 6 digits (see httpfeatures.GenerateCode)
	reResetCode = regexp.MustCompile(`^[0-9]{6}$`)
)

func ValidateNewUser(req *dto.RegisterNewUserRequest) error {
	return ozzoval.ValidateStruct(
		req,
		ozzoval.Field(&req.Email, ozzoval.Required, ozzoval.By(isValidEmail), ozzoval.Length(7, 60)),
		ozzoval.Field(&req.Password, ozzoval.Required, ozzoval.By(validatePassword)),
		ozzoval.Field(&req.Username, ozzoval.Required, ozzoval.Length(httpfeatures.UsernameMinLen, httpfeatures.UsernameMaxLen)),
	)
}

func ValidateLogin(req *dto.LoginUserRequest) error {
	return ozzoval.ValidateStruct(
		req,
		ozzoval.Field(&req.Email, ozzoval.Required, ozzoval.By(isValidEmail)),
		ozzoval.Field(&req.Password, ozzoval.Required),
	)
}

func ValidateForgotPass(req *dto.ForgotPasswordRequest) error {
	return ozzoval.ValidateStruct(
		req,
		ozzoval.Field(&req.Email, ozzoval.Required, ozzoval.By(isValidEmail)),
	)
}

// ValidateResetPass: same password rules as registration, so a reset can't set a weaker password.
func ValidateResetPass(req *dto.ResetPasswordRequest) error {
	return ozzoval.ValidateStruct(
		req,
		ozzoval.Field(&req.Email, ozzoval.Required, ozzoval.By(isValidEmail)),
		ozzoval.Field(&req.Code, ozzoval.Required, ozzoval.Match(reResetCode)),
		ozzoval.Field(&req.NewPassword, ozzoval.Required, ozzoval.By(validatePassword)),
	)
}

func ValidateVerifyEmail(req *dto.VerifyEmailRequest) error {
	return ozzoval.ValidateStruct(
		req,
		ozzoval.Field(&req.Code, ozzoval.Required, ozzoval.Match(reResetCode)),
	)
}

func isValidEmail(value interface{}) error {
	email, ok := value.(string)
	if !ok {
		return fmt.Errorf("email not a string")
	}
	if _, err := mail.ParseAddress(email); err != nil {
		return err
	}
	return nil
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
