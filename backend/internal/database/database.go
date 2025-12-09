package database

import (
	"fmt"
	"strings"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"updockly/backend/internal/config"
	"updockly/backend/internal/domain"
)

// Connect opens a GORM database connection and runs migrations.
func Connect(cfg config.Config) (*gorm.DB, error) {
	var dial gorm.Dialector
	lower := strings.ToLower(cfg.DatabaseURL)
	switch {
	case strings.HasPrefix(lower, "postgres://") || strings.HasPrefix(lower, "postgresql://"):
		dial = postgres.Open(cfg.DatabaseURL)
	case strings.HasPrefix(lower, "sqlite://"):
		dsn := strings.TrimPrefix(cfg.DatabaseURL, "sqlite://")
		dial = sqlite.Open(dsn)
	default:
		return nil, fmt.Errorf("DATABASE_URL is not set or has an invalid scheme")
	}

	db, err := gorm.Open(dial, &gorm.Config{Logger: logger.Default.LogMode(logger.Warn)})
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetMaxOpenConns(15)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)

	if err := db.AutoMigrate(
		&domain.Account{},
		&domain.ContainerSettings{},
		&domain.Schedule{},
		&domain.Agent{},
		&domain.AgentCommand{},
		&domain.UpdateHistory{},
	); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return db, nil
}
