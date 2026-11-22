package httphandler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/killerquinn/referral-system-go/internal/features/auth/dto"
	localmdw "github.com/killerquinn/referral-system-go/internal/infrastructure/http/middleware"
	httpfeatures "github.com/killerquinn/referral-system-go/internal/infrastructure/pkg/http-features"
	"github.com/killerquinn/referral-system-go/internal/infrastructure/pkg/jwtcontext"
	sharederrors "github.com/killerquinn/referral-system-go/internal/shared/shared-errors.go"
)

type AuthService interface {
	RegisterUser(ctx context.Context, username string, email string, password string) (userid string, err error)
	Login(ctx context.Context, email string, password string, userAgent string, userIP string) (actoken string, rtoken string, err error)
	Logout(ctx context.Context, rawRefresh string) error
	LogoutAll(ctx context.Context, userID string) error
	Refresh(ctx context.Context, rtokenHash string, metadata dto.SessionMetadata) (tokens *dto.Tokens, err error)
	ForgotPass(ctx context.Context, email string) (err error)
	ResetPassword(ctx context.Context, email, code, newPassword string) error
	SendEmailVerification(ctx context.Context, userID string) error
	VerifyEmail(ctx context.Context, userID, code string) error
}

type EmailLimiter interface {
	Allow(key string) bool
}

type HandlerRest struct {
	service      AuthService
	emailLimiter EmailLimiter
}

func NewAuthHandler(as AuthService, el EmailLimiter) *HandlerRest {
	return &HandlerRest{service: as, emailLimiter: el}
}

func Register(r chi.Router, as AuthService, ipLimiter *localmdw.RateLimiter, emailLimiter EmailLimiter) {
	h := NewAuthHandler(as, emailLimiter)

	r.Post("/auth/register", h.RegisterNewUser)
	r.With(ipLimiter.ByIP).Post("/auth/login", h.UserLogIn)
	r.Post("/auth/refresh", h.Refresh)
	r.Post("/auth/logout", h.UserLogOut)
	r.With(ipLimiter.ByIP).Post("/auth/forgot-password", h.ForgotPassword)
	r.With(ipLimiter.ByIP).Post("/auth/reset-password", h.ResetPassword)
}

var (
	ErrInvalidCreds = errors.New("invalid credentials")
)

func (h *HandlerRest) RegisterNewUser(w http.ResponseWriter, r *http.Request) {
	const op = "handler.RegisterNewUser"

	var req dto.RegisterNewUserRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responseReturn(w, http.StatusBadRequest, "bad request")
		return
	}

	if err := ValidateNewUser(&req); err != nil {
		responseReturn(w, http.StatusBadRequest, fmt.Sprintf("%s:%s", op, err.Error()))
		return
	}

	userID, err := h.service.RegisterUser(r.Context(), req.Username, req.Email, req.Password)
	if err != nil {
		if errors.Is(err, sharederrors.ErrNotUniqueUsername) {
			responseReturn(w, http.StatusForbidden, sharederrors.ErrNotUniqueUsername.Error())
			return
		}
		if errors.Is(err, sharederrors.ErrUserAlreadyRegistered) {
			responseReturn(w, http.StatusForbidden, sharederrors.ErrUserAlreadyRegistered.Error())
			return
		}
		responseReturn(w, http.StatusInternalServerError, "status internal error")
		return
	}

	responseReturn(w, http.StatusOK, userID)
}

func (h *HandlerRest) UserLogIn(w http.ResponseWriter, r *http.Request) {
	const op = "handler.Login"

	var req dto.LoginUserRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responseReturn(w, http.StatusBadRequest, "bad request")
		return
	}

	if err := ValidateLogin(&req); err != nil {
		responseReturn(w, http.StatusBadRequest, fmt.Sprintf("%s:%s", op, err.Error()))
		return
	}

	if !h.emailLimiter.Allow(strings.ToLower(req.Email)) {
		responseReturn(w, http.StatusTooManyRequests, "too many requests")
		return
	}

	userAgent := truncateUserAgent(r.UserAgent())

	ip := getClientIP(r)

	atoken, rtoken, err := h.service.Login(r.Context(), req.Email, req.Password, userAgent, ip)
	if err != nil {
		if errors.Is(err, sharederrors.ErrInvalidCreds) {
			responseReturn(w, http.StatusForbidden, "invalid creds")
			return
		}
		if errors.Is(err, sharederrors.ErrUserNotFound) {
			responseReturn(w, http.StatusNotFound, "user doesnt exist")
			return
		}
		if errors.Is(err, sharederrors.ErrUserIsBanned) {
			responseReturn(w, http.StatusForbidden, sharederrors.ErrUserIsBanned.Error())
			return
		}
		responseReturn(w, http.StatusInternalServerError, "status internal error")
		return
	}

	resp := dto.Tokens{
		Access:  atoken,
		Refresh: rtoken,
	}

	responseReturn(w, http.StatusOK, resp)
}

// UserLogOut ends only the session the given refresh token belongs to.
// It doesn't require an access token, so logout still works after it expired.
func (h *HandlerRest) UserLogOut(w http.ResponseWriter, r *http.Request) {
	const op = "handler.Logout"

	var req dto.RefreshRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Refresh == "" {
		responseReturn(w, http.StatusBadRequest, "bad request")
		return
	}

	if err := h.service.Logout(r.Context(), req.Refresh); err != nil {
		responseReturn(w, http.StatusInternalServerError, "status internal error")
		return
	}

	responseReturn(w, http.StatusOK, "successfull log out")
}

func (h *HandlerRest) UserLogOutAll(w http.ResponseWriter, r *http.Request) {
	const op = "handler.LogoutAll"

	userID, ok := jwtcontext.GetUserID(r.Context())
	if !ok {
		responseReturn(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	if err := h.service.LogoutAll(r.Context(), userID); err != nil {
		responseReturn(w, http.StatusInternalServerError, "status internal error")
		return
	}

	responseReturn(w, http.StatusOK, "successfull log out from all devices")
}

func (h *HandlerRest) Refresh(w http.ResponseWriter, r *http.Request) {
	const op = "auth/http-handler Refresh"
	var req dto.RefreshRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fmt.Printf("%s:%s", op, err.Error())
		responseReturn(w, http.StatusBadRequest, "bad request")
		return
	}

	meta := dto.SessionMetadata{
		UserAgent: truncateUserAgent(r.UserAgent()),
		ClientIP:  getClientIP(r),
	}

	tokens, err := h.service.Refresh(r.Context(), req.Refresh, meta)
	if errors.Is(err, sharederrors.ErrInvalidRefresh) {
		responseReturn(w, http.StatusUnauthorized, sharederrors.ErrInvalidRefresh.Error())
		return
	}
	if err != nil {
		responseReturn(w, http.StatusInternalServerError, "status internal error")
		return
	}

	resp := tokens

	responseReturn(w, http.StatusOK, resp)
}

func (h *HandlerRest) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	const op = "auth/http-handler ForgotPassword"
	var req dto.ForgotPasswordRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responseReturn(w, http.StatusBadRequest, "bad request")
		return
	}

	if err := ValidateForgotPass(&req); err != nil {
		responseReturn(w, http.StatusBadRequest, "bad request")
		return
	}

	if err := h.service.ForgotPass(r.Context(), req.Email); err != nil {
		responseReturn(w, http.StatusInternalServerError, "internal server error")
		return
	}

	responseReturn(w, http.StatusOK, "if an account with this email exists, a code has been sent")
}

func (h *HandlerRest) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var req dto.ResetPasswordRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responseReturn(w, http.StatusBadRequest, "bad request")
		return
	}

	// validation text is safe to show (e.g. "new_password: password must contain at least one digit"):
	// it's about the request itself, not about whether the account exists
	if err := ValidateResetPass(&req); err != nil {
		responseReturn(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.service.ResetPassword(r.Context(), req.Email, req.Code, req.NewPassword); err != nil {
		if errors.Is(err, sharederrors.ErrInvalidCode) {
			responseReturn(w, http.StatusBadRequest, "invalid or expired code")
			return
		}
		responseReturn(w, http.StatusInternalServerError, "internal server error")
		return
	}

	responseReturn(w, http.StatusOK, "password changed, please log in again")
}

// SendEmailVerification (re)sends the code to the logged-in user. Errors are honest here, see the service.
func (h *HandlerRest) SendEmailVerification(w http.ResponseWriter, r *http.Request) {
	userID, ok := jwtcontext.GetUserID(r.Context())
	if !ok {
		responseReturn(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	if err := h.service.SendEmailVerification(r.Context(), userID); err != nil {
		switch {
		case errors.Is(err, sharederrors.ErrEmailAlreadyVerified):
			responseReturn(w, http.StatusConflict, "email is already verified")
		case errors.Is(err, sharederrors.ErrCodeCooldown):
			responseReturn(w, http.StatusTooManyRequests, "wait a minute before requesting a new code")
		case errors.Is(err, sharederrors.ErrUserNotFound):
			responseReturn(w, http.StatusNotFound, "user not found")
		default:
			responseReturn(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	responseReturn(w, http.StatusOK, "verification code sent")
}

func (h *HandlerRest) VerifyEmail(w http.ResponseWriter, r *http.Request) {
	userID, ok := jwtcontext.GetUserID(r.Context())
	if !ok {
		responseReturn(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req dto.VerifyEmailRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responseReturn(w, http.StatusBadRequest, "bad request")
		return
	}

	if err := ValidateVerifyEmail(&req); err != nil {
		responseReturn(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.service.VerifyEmail(r.Context(), userID, req.Code); err != nil {
		switch {
		case errors.Is(err, sharederrors.ErrInvalidCode):
			responseReturn(w, http.StatusBadRequest, "invalid or expired code")
		case errors.Is(err, sharederrors.ErrEmailAlreadyVerified):
			responseReturn(w, http.StatusConflict, "email is already verified")
		case errors.Is(err, sharederrors.ErrUserNotFound):
			responseReturn(w, http.StatusNotFound, "user not found")
		default:
			responseReturn(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	responseReturn(w, http.StatusOK, "email verified")
}

// client IP is resolved by localmdw.ClientIPResolver; proxy headers are trusted only from configured proxies
func getClientIP(r *http.Request) string {
	return localmdw.ClientIP(r)
}

// user_sessions.user_agent is VARCHAR(255)
const maxUserAgentLen = 255

func truncateUserAgent(ua string) string {
	if utf8.RuneCountInString(ua) <= maxUserAgentLen {
		return ua
	}
	return string([]rune(ua)[:maxUserAgentLen])
}

func responseReturn(w http.ResponseWriter, status int, resp any, headers ...httpfeatures.Header) {
	w.Header().Set("Content-Type", "application/json")

	for _, h := range headers {
		w.Header().Set(h.Key, h.Value)
	}

	w.WriteHeader(status)
	json.NewEncoder(w).Encode(resp)
}
