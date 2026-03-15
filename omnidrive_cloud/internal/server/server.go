package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

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
	server := &http.Server{
		Addr:    cfg.BindAddr,
		Handler: apphttp.NewRouter(app),
	}

	cleanup := func() {
		db.Close()
	}

	return server, cleanup, nil
}
