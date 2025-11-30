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

	needsSetup := true
	if db != nil {
		var count int64
		if err := db.Model(&server.Account{}).Count(&count).Error; err == nil && count > 0 {
			needsSetup = false
		}
	}

	if isWeakSecret(cfg.SecretKey) {
		if needsSetup {
			if fresh, err := generateStrongSecret(); err == nil {
				settings := config.CurrentRuntimeSettings()
				settings.SecretKey = fresh
				if err := config.SaveRuntimeSettings(config.EnvFilePath, settings); err != nil {
					log.Printf("warning: generated SECRET_KEY but failed to persist to %s: %v", config.EnvFilePath, err)
				} else {
					cfg.SecretKey = fresh
					log.Println("INFO: generated a strong SECRET_KEY for initial setup and saved it to .env")
				}
			} else {
				log.Printf("warning: SECRET_KEY is weak and auto-generation failed: %v", err)
			}
		} else {
			log.Fatal("Refusing to start with weak/default SECRET_KEY; set a strong SECRET_KEY environment variable.")
		}
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
