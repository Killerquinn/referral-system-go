package app

import (
	"time"

	"github.com/killerquinn/referral-system-go/internal/config"
	authrepopackage "github.com/killerquinn/referral-system-go/internal/features/auth/repo"
	authservice "github.com/killerquinn/referral-system-go/internal/features/auth/service"
	userrepopackage "github.com/killerquinn/referral-system-go/internal/features/user/repo"
	userservice "github.com/killerquinn/referral-system-go/internal/features/user/service"
	"github.com/killerquinn/referral-system-go/internal/infrastructure/http/rest"
	"go.uber.org/zap"
)

type App struct {
	RestServer *rest.App
}

func New(logger *zap.Logger, port int, tokenTTL time.Duration, cfg *config.Config) *App {
	const op = "New"

	//AUTH INTERFACES

	authstorage := authrepopackage.New(cfg.Postgres.DSN)

	secret := cfg.JWT.Secret

	authService := authservice.New(logger,
		[]byte(secret),
		tokenTTL,
		authstorage, // userAuth interface
		authstorage, // newUser interface
		authstorage, // session interface
	)

	//USER INTERFACES

	userqueries := userrepopackage.New(cfg.Postgres.DSN)

	userService := userservice.New(
		logger,
		userqueries,
		userqueries,
	)

	//add referral feature there

	app := rest.NewApp(logger, cfg, authService, userService)

	return &App{
		RestServer: app,
	}
}
