package app

import (
	"omnidrive_cloud/internal/config"
	"omnidrive_cloud/internal/database"
	"omnidrive_cloud/internal/security"
	"omnidrive_cloud/internal/storage"
	"omnidrive_cloud/internal/store"
)

type App struct {
	Config   config.Config
	Database *database.Database
	Store    *store.Store
	Tokens   *security.TokenManager
	Storage  *storage.Service
}

func New(cfg config.Config, db *database.Database, storageService *storage.Service) *App {
	return &App{
		Config:   cfg,
		Database: db,
		Store:    store.New(db.Pool),
		Tokens:   security.NewTokenManager(cfg.JWTSecret, cfg.AccessTokenExpireMinutes),
		Storage:  storageService,
	}
}
