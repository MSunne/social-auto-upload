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
)

func main() {
	var deviceCode string

	flag.StringVar(&deviceCode, "device-code", "", "Device code to unbind")
	flag.Parse()

	deviceCode = strings.TrimSpace(deviceCode)
	if deviceCode == "" {
		log.Fatal("--device-code is required")
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

	var deviceID string
	var name string
	err = pool.QueryRow(ctx, `
		UPDATE devices
		SET owner_user_id = NULL,
		    is_enabled = FALSE,
		    default_reasoning_model = NULL,
		    default_chat_model = NULL,
		    default_image_model = NULL,
		    default_video_model = NULL,
		    updated_at = NOW()
		WHERE device_code = $1
		RETURNING id, name
	`, deviceCode).Scan(&deviceID, &name)
	if err != nil {
		log.Fatalf("unbind device: %v", err)
	}

	fmt.Printf("unbound device %s (%s) id=%s\n", deviceCode, name, deviceID)
}
