package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"updockly/backend/internal/config"
	"updockly/backend/internal/database"
	"updockly/backend/internal/httpapi"
)

func main() {
	cfg := config.Load()

	db, err := database.Connect(cfg)
	if err != nil {
		log.Printf("warning: could not connect to database, starting in setup mode: %v", err)
		db = nil
	}

	srv, err := httpapi.New(cfg, db)
	if err != nil {
		log.Fatalf("server bootstrap failed: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := srv.Run(ctx); err != nil {
		log.Fatalf("server exited: %v", err)
	}
}
