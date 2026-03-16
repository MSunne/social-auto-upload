package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"omnidrive_cloud/internal/ai"
	appstate "omnidrive_cloud/internal/app"
	"omnidrive_cloud/internal/config"
	"omnidrive_cloud/internal/database"
	apphttp "omnidrive_cloud/internal/http"
	"omnidrive_cloud/internal/storage"
)

func New(cfg config.Config) (*http.Server, func(), error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	db, err := database.New(ctx, cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("init database: %w", err)
	}

	storageService, err := storage.New(cfg)
	if err != nil {
		db.Close()
		return nil, nil, fmt.Errorf("init storage: %w", err)
	}

	app := appstate.New(cfg, db, storageService)
	if err := app.EnsureAdminBootstrap(ctx); err != nil {
		db.Close()
		return nil, nil, fmt.Errorf("ensure admin bootstrap: %w", err)
	}
	server := &http.Server{
		Addr:    cfg.BindAddr,
		Handler: apphttp.NewRouter(app),
	}

	cleanupFns := make([]func(), 0, 2)
	if cfg.AIWorkerEnabled {
		worker, err := ai.NewWorker(app)
		if err != nil {
			db.Close()
			return nil, nil, fmt.Errorf("init ai worker: %w", err)
		}
		stopWorker := worker.Start(context.Background())
		cleanupFns = append(cleanupFns, stopWorker)
	}

	cleanup := func() {
		for _, fn := range cleanupFns {
			if fn != nil {
				fn()
			}
		}
		db.Close()
	}

	return server, cleanup, nil
}
