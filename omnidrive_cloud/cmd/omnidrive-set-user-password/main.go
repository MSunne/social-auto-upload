package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"omnidrive_cloud/internal/config"
	"omnidrive_cloud/internal/security"
)

func main() {
	var email string
	var phone string
	var password string

	flag.StringVar(&email, "email", "", "User email")
	flag.StringVar(&phone, "phone", "", "User phone")
	flag.StringVar(&password, "password", "", "New password")
	flag.Parse()

	email = strings.ToLower(strings.TrimSpace(email))
	phone = strings.TrimSpace(phone)
	password = strings.TrimSpace(password)
	if (email == "" && phone == "") || password == "" {
		log.Fatal("one of --email/--phone and --password are required")
	}

	cfg := config.Load()
	if strings.TrimSpace(cfg.DatabaseDSN) == "" {
		log.Fatal("OMNIDRIVE_DATABASE_DSN is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, cfg.DatabaseDSN)
	if err != nil {
		log.Fatalf("connect database: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		log.Fatalf("ping database: %v", err)
	}

	hasher := security.NewTokenManager("password-reset", 60)
	passwordHash, err := hasher.HashPassword(password)
	if err != nil {
		log.Fatalf("hash password: %v", err)
	}

	query := `UPDATE users SET password_hash = $2, updated_at = NOW() WHERE email = $1`
	identifier := email
	if phone != "" {
		query = `UPDATE users SET password_hash = $2, updated_at = NOW() WHERE phone = $1`
		identifier = phone
	}

	tag, err := pool.Exec(ctx, query, identifier, passwordHash)
	if err != nil {
		log.Fatalf("update user password: %v", err)
	}
	if tag.RowsAffected() == 0 {
		log.Fatalf("user not found: %s", identifier)
	}

	fmt.Printf("updated password for %s\n", identifier)
}
