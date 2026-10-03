package rest

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/killerquinn/referral-system-go/internal/config"
	authhttphandler "github.com/killerquinn/referral-system-go/internal/features/auth/http-handler"
	httphandler "github.com/killerquinn/referral-system-go/internal/features/auth/http-handler"
	authservice "github.com/killerquinn/referral-system-go/internal/features/auth/service"
	referralhttphandler "github.com/killerquinn/referral-system-go/internal/features/referral/http"
	referralservice "github.com/killerquinn/referral-system-go/internal/features/referral/service"
	userhttphandler "github.com/killerquinn/referral-system-go/internal/features/user/http"
	userservice "github.com/killerquinn/referral-system-go/internal/features/user/service"
	localmdw "github.com/killerquinn/referral-system-go/internal/infrastructure/http/middleware"
	"go.uber.org/zap"
)

type App struct {
	server *http.Server
	router *chi.Mux
	port   string
}

func NewApp(log *zap.Logger, cfg *config.Config, auth *authservice.Auth, user *userservice.Uservice, referral *referralservice.RefService) *App {
	r := chi.NewRouter()

	log.Info("adding middleware")

	//middlewares

	r.Use(middleware.RequestID)

	r.Use(middleware.Logger)

	r.Use(middleware.Recoverer)

	r.Use(middleware.RealIP)

	newAuthHandler := httphandler.NewAuthHandler(auth)
	_ = userhttphandler.NewUserHandler(user)
	_ = referralhttphandler.NewRefferalHandler(referral)

	r.Group(func(r chi.Router) {
		r.Use(localmdw.AuthMiddleware(cfg.JWT.Secret))

		r.Delete("/auth/logout", newAuthHandler.UserLogOut)
	})

	log.Info("setup server")
	port := strconv.Itoa(cfg.Server.Port)

	server := &http.Server{
		Addr:              fmt.Sprintf(":%s", port),
		Handler:           r,
		ReadTimeout:       5 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       5 * time.Second,
		WriteTimeout:      5 * time.Second,
	}

	authhttphandler.Register(r, auth)
	userhttphandler.Register(r, user)
	referralhttphandler.Register(r, referral)

	return &App{
		server: server,
		router: r,
		port:   port,
	}
}

func (a *App) Run(logger *zap.Logger) error {
	const op = "Run"

	logger.Info("starting http server",
		zap.String("op", op),
		zap.String("addr", a.server.Addr),
	)

	if err := a.server.ListenAndServe(); err != nil {
		if errors.Is(err, http.ErrServerClosed) {
			logger.Info("server stopped gracefully", zap.String("op", op))
			return nil
		}
		return fmt.Errorf("failed to serve: %w", err)
	}
	return nil
}

func (a *App) Shutdown(logger *zap.Logger, ctx context.Context) error {
	const op = "Shutdown"
	logger.Info(op)
	if err := a.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("failed to shutdown: %w", err)
	}
	return nil
}
