package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/killerquinn/referral-system-go/internal/config"
	"github.com/killerquinn/referral-system-go/internal/infrastructure/database"
	"github.com/killerquinn/referral-system-go/internal/infrastructure/http/rest/app"
	"github.com/killerquinn/referral-system-go/internal/infrastructure/imaging"
	"github.com/killerquinn/referral-system-go/internal/infrastructure/storage/r2"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	envDev   = "development"
	envProd  = "production"
	envLocal = "local"
)

func main() {
	cfg := config.LoadConfig()
	// load config
	log := LoggerSetup(cfg.Server.Env)
	log.Info("Starting server...")
	log.Info("logger and config loaded")

	// cancelled on shutdown -> stops background jobs
	appCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	//load postgres
	postgres, err := database.NewPool(cfg)
	if err != nil {
		log.Error("failed to create postgres pool", zap.Error(err))
		os.Exit(1)
	}
	defer func(postgres *database.Postgres) {
		err := postgres.Close()
		if err != nil {
			log.Error("failed to close postgres pool", zap.Error(err))
		}
	}(postgres)

	// object storage (Cloudflare R2). nil = disabled: upload endpoints answer 503.
	storage, err := r2.New(cfg.R2)
	if err != nil {
		log.Warn("r2 storage disabled", zap.Error(err))
	} else {
		checkCtx, checkCancel := context.WithTimeout(context.Background(), 10*time.Second)
		if err := storage.CheckBuckets(checkCtx); err != nil {
			// buckets missing / wrong credentials -> disable instead of failing on every upload
			log.Error("r2 bucket check failed, storage disabled", zap.Error(err))
			storage = nil
		} else {
			log.Info("r2 storage connected",
				zap.String("public_bucket", storage.PublicBucket()),
				zap.String("private_bucket", storage.PrivateBucket()))
		}
		checkCancel()
	}

	//start server

	// libvips for image processing (upload jobs)
	if err := imaging.Startup(); err != nil {
		log.Error("failed to start libvips", zap.Error(err))
		os.Exit(1)
	}
	defer imaging.Shutdown()

	application, err := app.New(log, postgres.Pool, storage, cfg.Server.Port, cfg.JWT.TTL, cfg.Refresh.TTL, cfg)
	if err != nil {
		log.Error("failed to build application", zap.Error(err))
		os.Exit(1)
	}
	go func() {
		if err := application.RestServer.Run(log); err != nil {
			log.Fatal("server didn't start for some reason... : ", zap.Error(err))
		}
	}()

	// background jobs (River): upload processing + periodic cleanup
	if err := application.Queue.Start(appCtx); err != nil {
		log.Error("failed to start job queue", zap.Error(err))
		os.Exit(1)
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	//graceful shutdown

	stop := make(chan os.Signal, 1)

	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	siginfo := <-stop

	log.Info(fmt.Sprintf("application shutdowns on port %v, with syscall %v", cfg.Server.Port, siginfo.String()))

	application.RestServer.Shutdown(log, shutdownCtx)

	// let running jobs finish (until shutdownCtx expires), then cancel the rest
	if err := application.Queue.Stop(shutdownCtx); err != nil {
		log.Warn("job queue did not stop cleanly", zap.Error(err))
	}
	cancel()

	log.Info("database will close just after closing main")

	log.Info("app is closed successfully")
}

func LoggerSetup(env string) *zap.Logger {
	var level zap.AtomicLevel
	var development bool
	switch env {
	case envDev:
		level = zap.NewAtomicLevelAt(zap.ErrorLevel)
		development = true
	case envLocal:
		level = zap.NewAtomicLevelAt(zap.DebugLevel)
		development = true
	case envProd:
		level = zap.NewAtomicLevelAt(zap.InfoLevel)
		development = false
	}

	zapCfg := zap.Config{
		Level:            level,
		Development:      development,
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
		Sampling: &zap.SamplingConfig{
			Initial:    100,
			Thereafter: 100,
		},
		Encoding: "json",
		EncoderConfig: zapcore.EncoderConfig{
			TimeKey:        "timestamp",
			LevelKey:       "level",
			NameKey:        "logger",
			CallerKey:      "caller",
			MessageKey:     "message",
			StacktraceKey:  "stacktrace",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.LowercaseLevelEncoder,
			EncodeTime:     zapcore.ISO8601TimeEncoder,
			EncodeDuration: zapcore.SecondsDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		},
	}
	zapLogger, err := zapCfg.Build()
	if err != nil {
		fmt.Print("error in logger setup")
	}
	return zapLogger
}
