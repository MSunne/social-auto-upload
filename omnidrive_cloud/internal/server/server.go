package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"omnidrive_cloud/internal/ai"
	appstate "omnidrive_cloud/internal/app"
	"omnidrive_cloud/internal/config"
	"omnidrive_cloud/internal/database"
	apphttp "omnidrive_cloud/internal/http"
	"omnidrive_cloud/internal/storage"
	"omnidrive_cloud/internal/workflow"
)

func New(cfg config.Config, logger *slog.Logger) (*http.Server, func(), error) {
	if logger == nil {
		logger = slog.Default()
	}

	logger.Info("initializing omnidrive api",
		"bind_addr", cfg.BindAddr,
		"log_level", cfg.LogLevel,
		"ai_worker_enabled", cfg.AIWorkerEnabled,
		"ai_worker_concurrency", cfg.AIWorkerConcurrency,
		"cors_origin_count", len(cfg.CORSAllowedOrigins),
		"storage_mode_hint", storageModeHint(cfg),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	logger.Debug("connecting database")
	db, err := database.New(ctx, cfg, logger)
	if err != nil {
		return nil, nil, fmt.Errorf("init database: %w", err)
	}
	logger.Info("database initialized")

	storageService, err := storage.New(cfg)
	if err != nil {
		db.Close()
		return nil, nil, fmt.Errorf("init storage: %w", err)
	}
	logger.Info("storage initialized", "mode", storageService.Mode())

	app := appstate.New(cfg, db, storageService, logger)
	if err := app.EnsureAdminBootstrap(ctx); err != nil {
		db.Close()
		return nil, nil, fmt.Errorf("ensure admin bootstrap: %w", err)
	}
	logger.Debug("admin bootstrap ensured")
	if err := app.EnsureDevelopmentSeedUsers(ctx); err != nil {
		db.Close()
		return nil, nil, fmt.Errorf("ensure development seed users: %w", err)
	}
	logger.Debug("development seed users ensured")
	server := &http.Server{
		Addr:    cfg.BindAddr,
		Handler: apphttp.NewRouter(app),
	}

	cleanupFns := make([]func(), 0, 4)
	if cfg.AIWorkerEnabled {
		worker, err := ai.NewWorker(app)
		if err != nil {
			db.Close()
			return nil, nil, fmt.Errorf("init ai worker: %w", err)
		}
		stopWorker := worker.Start(context.Background())
		cleanupFns = append(cleanupFns, stopWorker)
	} else {
		logger.Info("ai worker disabled")
	}

	skillScheduler, err := workflow.NewSkillScheduler(app)
	if err != nil {
		db.Close()
		return nil, nil, fmt.Errorf("init skill scheduler: %w", err)
	}
	stopSkillScheduler := skillScheduler.Start(context.Background())
	cleanupFns = append(cleanupFns, stopSkillScheduler)

	quotaExpiryScheduler, err := workflow.NewQuotaExpiryScheduler(app)
	if err != nil {
		db.Close()
		return nil, nil, fmt.Errorf("init quota expiry scheduler: %w", err)
	}
	stopQuotaExpiryScheduler := quotaExpiryScheduler.Start(context.Background())
	cleanupFns = append(cleanupFns, stopQuotaExpiryScheduler)

	cleanup := func() {
		logger.Info("shutting down omnidrive api")
		for _, fn := range cleanupFns {
			if fn != nil {
				fn()
			}
		}
		db.Close()
		logger.Info("omnidrive api cleanup complete")
	}

	return server, cleanup, nil
}

func storageModeHint(cfg config.Config) string {
	if strings.TrimSpace(cfg.S3Endpoint) != "" &&
		strings.TrimSpace(cfg.S3Bucket) != "" &&
		strings.TrimSpace(cfg.S3AccessKey) != "" &&
		strings.TrimSpace(cfg.S3SecretKey) != "" {
		return "s3"
	}
	return "local"
}
