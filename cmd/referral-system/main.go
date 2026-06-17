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

	_, cancel := context.WithCancel(context.Background())
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

	//start server

	application := app.New(log, cfg.Server.Port, time.Minute*15, cfg)
	go func() {
		if err := application.RestServer.Run(log); err != nil {
			log.Fatal("server didn't start for some reason... : ", zap.Error(err))
		}
	}()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	//graceful shutdown

	stop := make(chan os.Signal, 1)

	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	siginfo := <-stop

	log.Info(fmt.Sprintf("application shutdowns on port %v, with syscall %v", cfg.Server.Port, siginfo.String()))

	application.RestServer.Shutdown(log, shutdownCtx)

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
		Level:       level,
		Development: development,
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
	zapLogger, _ := zapCfg.Build()
	return zapLogger
}
