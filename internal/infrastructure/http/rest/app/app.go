package app

import (
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/killerquinn/referral-system-go/internal/config"
	authrepopackage "github.com/killerquinn/referral-system-go/internal/features/auth/repo"
	authservice "github.com/killerquinn/referral-system-go/internal/features/auth/service"
	pagerepopackage "github.com/killerquinn/referral-system-go/internal/features/page/repo"
	pageservice "github.com/killerquinn/referral-system-go/internal/features/page/service"
	postrepopackage "github.com/killerquinn/referral-system-go/internal/features/post/repo"
	postservice "github.com/killerquinn/referral-system-go/internal/features/post/service"
	referralrepopackage "github.com/killerquinn/referral-system-go/internal/features/referral/repo"
	referralservice "github.com/killerquinn/referral-system-go/internal/features/referral/service"
	uploadrepopackage "github.com/killerquinn/referral-system-go/internal/features/upload/repo"
	uploadservice "github.com/killerquinn/referral-system-go/internal/features/upload/service"
	userrepopackage "github.com/killerquinn/referral-system-go/internal/features/user/repo"
	userservice "github.com/killerquinn/referral-system-go/internal/features/user/service"
	"github.com/killerquinn/referral-system-go/internal/infrastructure/http/rest"
	"github.com/killerquinn/referral-system-go/internal/infrastructure/queue"
	"github.com/killerquinn/referral-system-go/internal/infrastructure/storage/r2"
	"github.com/killerquinn/referral-system-go/internal/shared/media"
	"go.uber.org/zap"
)

// how often an author can change their page address
const pageSlugCooldown = 30 * 24 * time.Hour

type App struct {
	RestServer *rest.App
	Queue      *queue.Queue // background jobs: started / stopped in main
}

func New(logger *zap.Logger, pool *pgxpool.Pool, storage *r2.Client, port int, tokenTTL time.Duration, refreshTTL time.Duration, cfg *config.Config) (*App, error) {
	const op = "New"

	// public file URLs for avatars / page media. UNTYPED nil when R2 is disabled (see media.URLBuilder).
	var urls media.URLBuilder
	if storage != nil {
		urls = storage
	}

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

	//UPLOAD INTERFACES

	logger.Info("start loading upload interfaces")

	uploadrepo := uploadrepopackage.New(pool)

	jobs, err := queue.New(logger, pool)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	// storage must reach the service as an UNTYPED nil when R2 is disabled: a nil *r2.Client
	// wrapped in the interface is != nil and the service's nil checks would pass.
	var uploadService *uploadservice.UploadService
	if storage != nil {
		uploadService = uploadservice.New(logger, uploadrepo, storage, jobs)
	} else {
		uploadService = uploadservice.New(logger, uploadrepo, nil, jobs)
	}
	// workers call back into the service (process / cleanup)
	jobs.SetUploadProcessor(uploadService)

	//USER INTERFACES

	logger.Info("start loading user interfaces")

	userqueries := userrepopackage.New(pool)

	userService := userservice.New(
		logger,
		userqueries,
		userqueries,
		userqueries,
		userqueries,
		userqueries, // avatars
		urls,
		uploadService, // deletes the previous avatar file when it's replaced
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

	var mediaSigner postservice.MediaSigner
	if storage != nil {
		mediaSigner = storage
	}

	postservice := postservice.New(
		logger,
		cfg.Options.BaseUrl,
		postrepo,
		urls,
		mediaSigner,
	)

	//PAGE INTERFACES

	pagerepo := pagerepopackage.New(pool)

	pageService := pageservice.New(
		logger,
		cfg.Options.FrontendURL,
		pageSlugCooldown,
		pagerepo,
		urls,
		uploadService,      // deletes the previous background / music file when it's replaced
		[]byte(hmacsecret), // keys the daily visitor hashes of the view counter
	)
	jobs.SetViewsCleaner(pageService)

	app := rest.NewApp(logger, cfg, authService, userService, referralService, postservice, uploadService, pageService)

	return &App{
		RestServer: app,
		Queue:      jobs,
	}, nil
}
