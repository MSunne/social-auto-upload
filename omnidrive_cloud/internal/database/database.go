package database

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"omnidrive_cloud/internal/config"
)

type Database struct {
	Pool   *pgxpool.Pool
	Logger *slog.Logger
}

// 创建数据库相关实例，组装运行所需依赖并返回给上层流程复用。
func New(ctx context.Context, cfg config.Config, logger *slog.Logger) (*Database, error) {
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
	poolConfig.ConnConfig.Tracer = newQueryTracer(logger)

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("connect database: %w", err)
	}

	db := &Database{Pool: pool, Logger: logger}
	if cfg.AutoCreateSchema {
		if err := db.EnsureSchema(ctx); err != nil {
			pool.Close()
			return nil, err
		}
	}
	return db, nil
}

// 处理Close相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func (db *Database) Close() {
	if db != nil && db.Pool != nil {
		db.Pool.Close()
	}
}
