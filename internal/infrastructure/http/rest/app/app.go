package app

import (
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/killerquinn/referral-system-go/internal/config"
	"github.com/killerquinn/referral-system-go/internal/features/user"
	"github.com/killerquinn/referral-system-go/internal/infrastructure/http/rest"
	v1 "github.com/killerquinn/referral-system-go/internal/infrastructure/http/v1"
	"go.uber.org/zap"
)

type App struct {
	RestApp rest.App
}

func New(logger *zap.Logger, port int, tokenTTL time.Duration, cfg *config.Config) *App {
	const op = "New"
	restApp := rest.NewApp(logger, cfg)

	return &App{}
}
