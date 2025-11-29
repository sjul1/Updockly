package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"updockly/backend/internal/config"
	"updockly/backend/internal/database"
	"updockly/backend/internal/server"
)

func main() {
	cfg := config.Load()

	// Security check for production environments
	if cfg.SecretKey == "dev-secret-key" || len(cfg.SecretKey) < 32 {
		log.Println("CRITICAL WARNING: You are using a weak or default SECRET_KEY.")
		log.Println("This is unsafe for production. Please set a strong SECRET_KEY environment variable.")
		// In strict mode we could os.Exit(1), but for now we warn aggressively.
	}

	db, err := database.Connect(cfg)
	if err != nil {
		log.Printf("warning: could not connect to database, starting in setup mode: %v", err)
		db = nil
	}

	srv, err := server.New(cfg, db)
	if err != nil {
		log.Fatalf("server bootstrap failed: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := srv.Run(ctx); err != nil {
		log.Fatalf("server exited: %v", err)
	}
}
