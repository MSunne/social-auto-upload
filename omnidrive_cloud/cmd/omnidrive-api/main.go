package main

import (
	"log"

	"omnidrive_cloud/internal/config"
	"omnidrive_cloud/internal/server"
)

func main() {
	cfg := config.Load()
	srv, cleanup, err := server.New(cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer cleanup()

	log.Printf("omnidrive api listening on %s", cfg.BindAddr)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
