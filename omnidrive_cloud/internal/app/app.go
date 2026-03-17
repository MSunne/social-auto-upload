package app

import (
	"log/slog"

	"omnidrive_cloud/internal/config"
	"omnidrive_cloud/internal/database"
	"omnidrive_cloud/internal/security"
	"omnidrive_cloud/internal/storage"
	"omnidrive_cloud/internal/store"
)

type App struct {
	Config      config.Config
	Database    *database.Database
	Store       *store.Store
	Tokens      *security.TokenManager
	AdminTokens *security.TokenManager
	Storage     *storage.Service
	Logger      *slog.Logger
}

func New(cfg config.Config, db *database.Database, storageService *storage.Service, logger *slog.Logger) *App {
	if logger == nil {
		logger = slog.Default()
	}

	return &App{
		Config:      cfg,
		Database:    db,
		Store:       store.New(db.Pool),
		Tokens:      security.NewTokenManager(cfg.JWTSecret, cfg.AccessTokenExpireMinutes),
		AdminTokens: security.NewTokenManager(cfg.AdminJWTSecret, cfg.AdminAccessTokenExpireMinutes),
		Storage:     storageService,
		Logger:      logger,
	}
}
