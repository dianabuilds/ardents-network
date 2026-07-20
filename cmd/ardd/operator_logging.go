package main

import (
	"context"
	"log/slog"
	"os"

	runtimeconfig "ardents/internal/runtime/config"
)

type loggingApplier struct {
	level *slog.LevelVar
}

func configureOperatorLogging(cfg runtimeconfig.LoggingConfig) *loggingApplier {
	level := &slog.LevelVar{}
	level.Set(operatorLogLevel(cfg.Level))
	options := &slog.HandlerOptions{Level: level}
	var handler slog.Handler
	if cfg.Format == "text" {
		handler = slog.NewTextHandler(os.Stderr, options)
	} else {
		handler = slog.NewJSONHandler(os.Stderr, options)
	}
	slog.SetDefault(slog.New(handler))
	return &loggingApplier{level: level}
}

func (*loggingApplier) Prepare(context.Context, runtimeconfig.Document, runtimeconfig.Document) error {
	return nil
}

func (a *loggingApplier) Apply(_ context.Context, _ runtimeconfig.Document, next runtimeconfig.Document) error {
	a.level.Set(operatorLogLevel(next.Logging.Level))
	return nil
}

func (a *loggingApplier) Rollback(_ context.Context, previous runtimeconfig.Document) error {
	a.level.Set(operatorLogLevel(previous.Logging.Level))
	return nil
}

func operatorLogLevel(value string) slog.Level {
	switch value {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
