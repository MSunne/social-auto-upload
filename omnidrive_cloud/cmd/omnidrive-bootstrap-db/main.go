package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/url"
	"path"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"omnidrive_cloud/internal/config"
	"omnidrive_cloud/internal/database"
	"omnidrive_cloud/internal/logging"
)

// 组装数据库初始化命令依赖并启动主流程，发生错误时直接退出进程。
func main() {
	cfg := config.Load()
	if cfg.DatabaseDSN == "" {
		log.Fatal("OMNIDRIVE_DATABASE_DSN is required")
	}

	logger := logging.New(cfg)
	slog.SetDefault(logger)

	targetURL, err := url.Parse(cfg.DatabaseDSN)
	if err != nil {
		log.Fatalf("parse dsn: %v", err)
	}
	targetDBName := path.Base(targetURL.Path)
	if targetDBName == "" || targetDBName == "." || targetDBName == "/" {
		log.Fatal("database name is missing from OMNIDRIVE_DATABASE_DSN")
	}
	if !regexp.MustCompile(`^[a-zA-Z0-9_]+$`).MatchString(targetDBName) {
		log.Fatal("database name may only contain letters, numbers, and underscore")
	}

	adminURL := *targetURL
	adminURL.Path = "/postgres"

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	targetConn, err := pgx.Connect(ctx, cfg.DatabaseDSN)
	targetExists := err == nil
	if targetExists {
		targetConn.Close(ctx)
		fmt.Printf("database %q already exists\n", targetDBName)
	} else {
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "3D000" {
			log.Fatalf("connect target database: %v", err)
		}

		conn, adminErr := pgx.Connect(ctx, adminURL.String())
		if adminErr != nil {
			log.Fatalf("connect postgres admin db: %v", adminErr)
		}
		defer conn.Close(ctx)

		var exists bool
		if queryErr := conn.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname = $1)`, targetDBName).Scan(&exists); queryErr != nil {
			log.Fatalf("check database exists: %v", queryErr)
		}
		if !exists {
			if _, execErr := conn.Exec(ctx, fmt.Sprintf(`CREATE DATABASE "%s"`, targetDBName)); execErr != nil {
				log.Fatalf("create database: %v", execErr)
			}
			fmt.Printf("database %q created\n", targetDBName)
		} else {
			fmt.Printf("database %q already exists\n", targetDBName)
		}
	}

	cfg.AutoCreateSchema = true
	schemaCtx, schemaCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer schemaCancel()

	db, err := database.New(schemaCtx, cfg, logger)
	if err != nil {
		log.Fatalf("ensure schema: %v", err)
	}
	defer db.Close()

	fmt.Printf("schema ensured for %q\n", targetDBName)
}
