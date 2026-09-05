package app

import (
	"time"

	"github.com/killerquinn/referral-system-go/internal/config"
	repopackage "github.com/killerquinn/referral-system-go/internal/features/auth/repo"
	service "github.com/killerquinn/referral-system-go/internal/features/auth/service"
	"github.com/killerquinn/referral-system-go/internal/infrastructure/http/rest"
	"go.uber.org/zap"
)

type App struct {
	RestServer *rest.App
}

func New(logger *zap.Logger, port int, tokenTTL time.Duration, cfg *config.Config) *App {
	const op = "New"

	storage := repopackage.New(cfg.Postgres.DSN)

	authService := service.New(logger,
		storage, // userAuth interface
		storage, // newUser interface
	)

	app := rest.NewApp(logger, cfg, authService)

	return &App{
		RestServer: app,
	}
}
