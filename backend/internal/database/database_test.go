package database

import (
	"testing"

	"updockly/backend/internal/config"
)

func TestConnectSQLite(t *testing.T) {
	cfg := config.Config{
		DatabaseURL: "sqlite://:memory:",
	}

	db, err := Connect(cfg)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("failed to get underlying sql.DB: %v", err)
	}
	if err := sqlDB.Ping(); err != nil {
		t.Fatalf("ping failed: %v", err)
	}

	// Check if migrations ran by querying a table
	if !db.Migrator().HasTable("accounts") {
		t.Error("expected 'accounts' table to exist after migration")
	}
	if !db.Migrator().HasTable("container_settings") {
		t.Error("expected 'container_settings' table to exist after migration")
	}
	if !db.Migrator().HasTable("schedules") {
		t.Error("expected 'schedules' table to exist after migration")
	}
}

func TestConnectInvalidScheme(t *testing.T) {
	cfg := config.Config{
		DatabaseURL: "mysql://user:pass@localhost/db",
	}

	_, err := Connect(cfg)
	if err == nil {
		t.Fatal("expected error for unsupported scheme, got nil")
	}
}
