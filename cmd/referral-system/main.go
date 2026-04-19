package main

import (
	"github.com/killerquinn/referral-system-go/internal/config"
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
