package main

import (
	"errors"
	"log/slog"
	"net/http"
	"os"

	"omnidrive_cloud/internal/config"
	"omnidrive_cloud/internal/logging"
	"omnidrive_cloud/internal/server"
)

func main() {
	cfg := config.Load()
	logger := logging.New(cfg)
	slog.SetDefault(logger)

	srv, cleanup, err := server.New(cfg, logger)
	if err != nil {
		logger.Error("failed to initialize omnidrive api", "error", err)
		os.Exit(1)
	}
	defer cleanup()

	logger.Info("omnidrive api listening", "bind_addr", cfg.BindAddr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("omnidrive api stopped with error", "error", err)
		os.Exit(1)
	}

	logger.Info("omnidrive api stopped")
}
