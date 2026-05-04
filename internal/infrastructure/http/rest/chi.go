package rest

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/killerquinn/referral-system-go/internal/config"
	"go.uber.org/zap"
)

type App struct {
	server *http.Server
	router *chi.Mux
	port   string
}

func NewApp(log *zap.Logger, cfg *config.Config) *App {
	r := chi.NewRouter()

	log.Info("adding middleware")
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)

	log.Info("setup server")

	server := &http.Server{
		Addr:              cfg.Server.Port,
		Handler:           r,
		ReadTimeout:       5 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       5 * time.Second,
		WriteTimeout:      5 * time.Second,
	}

	return &App{
		server: server,
		router: r,
		port:   cfg.Server.Port,
	}
}

func (a *App) Run(logger *zap.Logger) error {
	const op = "Run"
	logger.Info(op, zap.String("server starting on", a.port))

	if err := a.server.ListenAndServe(); err != nil {
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
