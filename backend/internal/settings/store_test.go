package settings

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"updockly/backend/internal/config"
)

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&Record{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestStoreLoadEmptyDoesNotError(t *testing.T) {
	db := setupTestDB(t)
	store := NewStore(db)

	_, found, err := store.Load()
	if err != nil {
		t.Fatalf("load empty: %v", err)
	}
	if found {
		t.Fatalf("expected no record found for empty store")
	}
}

func TestStoreSaveStripsEnvBackedFields(t *testing.T) {
	db := setupTestDB(t)
	store := NewStore(db)

	settings := config.RuntimeSettings{
		DatabaseURL:  "postgres://should-not-persist",
		ClientOrigin: "http://ui.invalid",
		Timezone:     "Europe/Paris",
		HideSupport:  true,
		Notifications: config.NotificationSettings{
			WebhookURL: "https://hooks.example",
		},
	}

	if _, err := store.Save(settings); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, found, err := store.Load()
	if err != nil {
		t.Fatalf("load after save: %v", err)
	}
	if !found {
		t.Fatalf("expected record to be found after save")
	}
	if loaded.DatabaseURL != "" || loaded.ClientOrigin != "" {
		t.Fatalf("env-backed fields should be stripped, got db=%q client=%q", loaded.DatabaseURL, loaded.ClientOrigin)
	}
	if loaded.Timezone != "Europe/Paris" || !loaded.HideSupport || loaded.Notifications.WebhookURL != "https://hooks.example" {
		t.Fatalf("unexpected stored payload: %+v", loaded)
	}
}
