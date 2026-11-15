package app

import (
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/killerquinn/referral-system-go/internal/config"
	authrepopackage "github.com/killerquinn/referral-system-go/internal/features/auth/repo"
	authservice "github.com/killerquinn/referral-system-go/internal/features/auth/service"
	postrepopackage "github.com/killerquinn/referral-system-go/internal/features/post/repo"
	postservice "github.com/killerquinn/referral-system-go/internal/features/post/service"
	referralrepopackage "github.com/killerquinn/referral-system-go/internal/features/referral/repo"
	referralservice "github.com/killerquinn/referral-system-go/internal/features/referral/service"
	uploadrepopackage "github.com/killerquinn/referral-system-go/internal/features/upload/repo"
	uploadservice "github.com/killerquinn/referral-system-go/internal/features/upload/service"
	userrepopackage "github.com/killerquinn/referral-system-go/internal/features/user/repo"
	userservice "github.com/killerquinn/referral-system-go/internal/features/user/service"
	"github.com/killerquinn/referral-system-go/internal/infrastructure/http/rest"
	"github.com/killerquinn/referral-system-go/internal/infrastructure/storage/r2"
	"go.uber.org/zap"
)

type App struct {
	RestServer *rest.App
}

func New(logger *zap.Logger, pool *pgxpool.Pool, storage *r2.Client, port int, tokenTTL time.Duration, refreshTTL time.Duration, cfg *config.Config) *App {
	const op = "New"

	//AUTH INTERFACES

	logger.Info("start loading auth interfaces")

	authstorage := authrepopackage.New(pool)

	secret := cfg.JWT.Secret
	hmacsecret := cfg.Refresh.HMACSecret

	authService := authservice.New(logger,
		[]byte(secret),
		[]byte(hmacsecret),
		tokenTTL,
		refreshTTL,
		authstorage, // userAuth interface
		authstorage, // newUser interface
		authstorage, // session interface
	)

	//USER INTERFACES

	logger.Info("start loading user interfaces")

	userqueries := userrepopackage.New(pool)

	userService := userservice.New(
		logger,
		userqueries,
		userqueries,
		userqueries,
		userqueries,
	)

	//add referral feature there

	logger.Info("start loading referral interfaces")

	referralrepo := referralrepopackage.New(pool)

	referralService := referralservice.New(
		logger,
		cfg.Options.BaseUrl,
		referralrepo,
	)

	postrepo := postrepopackage.New(pool)

	postservice := postservice.New(
		logger,
		cfg.Options.BaseUrl,
		postrepo,
	)

	//UPLOAD INTERFACES

	logger.Info("start loading upload interfaces")

	uploadrepo := uploadrepopackage.New(pool)

	// storage must reach the service as an UNTYPED nil when R2 is disabled: a nil *r2.Client
	// wrapped in the interface is != nil and the service's nil checks would pass.
	// queue is nil until River is set up.
	var uploadService *uploadservice.UploadService
	if storage != nil {
		uploadService = uploadservice.New(logger, uploadrepo, storage, nil)
	} else {
		uploadService = uploadservice.New(logger, uploadrepo, nil, nil)
	}

	app := rest.NewApp(logger, cfg, authService, userService, referralService, postservice, uploadService)

	return &App{
		RestServer: app,
	}
}
