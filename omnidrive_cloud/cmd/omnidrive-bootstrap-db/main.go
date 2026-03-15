package main

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"path"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5"

	"omnidrive_cloud/internal/config"
)

func main() {
	cfg := config.Load()
	if cfg.DatabaseDSN == "" {
		log.Fatal("OMNIDRIVE_DATABASE_DSN is required")
	}

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

	conn, err := pgx.Connect(ctx, adminURL.String())
	if err != nil {
		log.Fatalf("connect postgres admin db: %v", err)
	}
	defer conn.Close(ctx)

	var exists bool
	if err := conn.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname = $1)`, targetDBName).Scan(&exists); err != nil {
		log.Fatalf("check database exists: %v", err)
	}

	if exists {
		fmt.Printf("database %q already exists\n", targetDBName)
		return
	}

	if _, err := conn.Exec(ctx, fmt.Sprintf(`CREATE DATABASE "%s"`, targetDBName)); err != nil {
		log.Fatalf("create database: %v", err)
	}

	fmt.Printf("database %q created\n", targetDBName)
}
