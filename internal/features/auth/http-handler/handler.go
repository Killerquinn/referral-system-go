package httphandler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/killerquinn/referral-system-go/internal/features/auth/dto"
	httpfeatures "github.com/killerquinn/referral-system-go/internal/infrastructure/pkg/http-features"
	"github.com/killerquinn/referral-system-go/internal/infrastructure/pkg/jwtcontext"
	sharederrors "github.com/killerquinn/referral-system-go/internal/shared/shared-errors.go"
)

type AuthService interface {
	RegisterUser(ctx context.Context, username string, email string, password string) (userid string, err error)
	Login(ctx context.Context, email string, password string, userAgent string, userIP string) (actoken string, rtoken string, err error)
	Logout(ctx context.Context, userID string) error
}

type HandlerRest struct {
	service AuthService
}

func NewAuthHandler(as AuthService) *HandlerRest {
	return &HandlerRest{service: as}
}

func Register(r chi.Router, as AuthService) {
	h := NewAuthHandler(as)

	r.Post("/auth/register", h.RegisterNewUser)
	r.Post("/auth/login", h.UserLogIn)
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

	userAgent := r.Header.Get("User-Agent")

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
		responseReturn(w, http.StatusInternalServerError, "status internal error")
		return
	}

	resp := dto.Tokens{
		Access:  atoken,
		Refresh: rtoken,
	}

	responseReturn(w, http.StatusOK, resp)
}

func (h *HandlerRest) UserLogOut(w http.ResponseWriter, r *http.Request) {
	const op = "handler.Logout"

	userID, ok := r.Context().Value(jwtcontext.UserIDKey).(string)
	if !ok {
		responseReturn(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	if err := h.service.Logout(r.Context(), userID); err != nil {
		responseReturn(w, http.StatusInternalServerError, "status internal error")
		return
	}

	responseReturn(w, http.StatusOK, "successfull log out")

}

func getClientIP(r *http.Request) string {
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		return ip
	}
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func responseReturn(w http.ResponseWriter, status int, resp any, headers ...httpfeatures.Header) {
	w.Header().Set("Content-Type", "application/json")

	for _, h := range headers {
		w.Header().Set(h.Key, h.Value)
	}

	w.WriteHeader(status)
	json.NewEncoder(w).Encode(resp)
}
