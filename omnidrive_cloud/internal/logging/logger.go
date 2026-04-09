package logging

import (
	"log/slog"
	"os"
	"strings"

	"omnidrive_cloud/internal/config"
)

// 创建日志相关实例，组装运行所需依赖并返回给上层流程复用。
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

// 解析Level，为日志提供结构化输入。
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
