package app

import (
	"time"

	"github.com/killerquinn/referral-system-go/internal/config"
	"github.com/killerquinn/referral-system-go/internal/infrastructure/http/rest"
	"go.uber.org/zap"
)

type App struct {
	Server *rest.App
}

func New(logger *zap.Logger, port int, tokenTTL time.Duration, cfg *config.Config) *App {
	const op = "New"

	return &App{}
}
