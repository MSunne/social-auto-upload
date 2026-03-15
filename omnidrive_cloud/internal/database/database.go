package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"omnidrive_cloud/internal/config"
)

type Database struct {
	Pool *pgxpool.Pool
}

func New(ctx context.Context, cfg config.Config) (*Database, error) {
	if cfg.DatabaseDSN == "" {
		return nil, fmt.Errorf("OMNIDRIVE_DATABASE_DSN is required")
	}

	poolConfig, err := pgxpool.ParseConfig(cfg.DatabaseDSN)
	if err != nil {
		return nil, fmt.Errorf("parse database dsn: %w", err)
	}
	poolConfig.MaxConns = 12
	poolConfig.MinConns = 1
	poolConfig.MaxConnIdleTime = 5 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("connect database: %w", err)
	}

	db := &Database{Pool: pool}
	if cfg.AutoCreateSchema {
		if err := db.EnsureSchema(ctx); err != nil {
			pool.Close()
			return nil, err
		}
	}
	return db, nil
}

func (db *Database) Close() {
	if db != nil && db.Pool != nil {
		db.Pool.Close()
	}
}
