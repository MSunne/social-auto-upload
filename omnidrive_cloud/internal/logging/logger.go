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

	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level:     parseLevel(cfg.LogLevel),
		AddSource: strings.EqualFold(environment, "development"),
	})

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
