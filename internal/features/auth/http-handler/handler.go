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
)

type AuthService interface {
	RegisterUser(ctx context.Context, username string, email string, password string) (userid string, err error)
	Login(ctx context.Context, email string, password string, userAgent string, userIP string) (actoken string, rtoken string, err error)
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
		http.Error(w, fmt.Sprintf("%s:%s", op, "invalid body"), http.StatusBadRequest)
		return
	}

	userID, err := h.service.RegisterUser(r.Context(), req.Username, req.Email, req.Password)
	if err != nil {
		http.Error(w, fmt.Sprintf("%s:%s", op, "internal status error"), http.StatusInternalServerError)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"user_id": userID})
}

func (h *HandlerRest) UserLogIn(w http.ResponseWriter, r *http.Request) {
	const op = "handler.Login"

	var req dto.LoginUserRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("%s:%s", op, "invalid body"), http.StatusBadRequest)
		return
	}

	userAgent := r.Header.Get("User-Agent")

	ip := getClientIP(r)

	atoken, rtoken, err := h.service.Login(r.Context(), req.Email, req.Password, userAgent, ip)
	if err != nil {
		if errors.Is(err, ErrInvalidCreds) {
			http.Error(w, "invalid credentials", http.StatusUnauthorized)
			return
		}
		http.Error(w, fmt.Sprintf("%s:%s", op, "internal status error"), http.StatusInternalServerError)
		return
	}

	resp := dto.Tokens{
		Access:  atoken,
		Refresh: rtoken,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
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
