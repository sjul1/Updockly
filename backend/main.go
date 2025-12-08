package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"log"
	"os/signal"
	"syscall"

	"updockly/backend/internal/config"
	"updockly/backend/internal/database"
	"updockly/backend/internal/server"
)

func main() {
	cfg := config.Load()

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

func isWeakSecret(secret string) bool {
	return secret == "dev-secret-key" || len(secret) < 32
}

func generateStrongSecret() (string, error) {
	buf := make([]byte, 48)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
