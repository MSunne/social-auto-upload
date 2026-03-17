package logging

import (
	"log/slog"
	"os"
	"strings"

	"omnidrive_cloud/internal/config"
)

func New(cfg config.Config) *slog.Logger {
	environment := strings.TrimSpace(cfg.Environment)
	if environment == "" {
		environment = "development"
	}

	handlerOptions := &slog.HandlerOptions{
		Level:     parseLevel(cfg.LogLevel),
		AddSource: strings.EqualFold(environment, "development"),
	}

	var handler slog.Handler
	switch strings.ToLower(strings.TrimSpace(cfg.LogFormat)) {
	case "text":
		handler = slog.NewTextHandler(os.Stdout, handlerOptions)
	default:
		handler = slog.NewJSONHandler(os.Stdout, handlerOptions)
	}

	return slog.New(handler).With(
		"service", "omnidrive-api",
		"environment", environment,
	)
}

func parseLevel(raw string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
