package auth

import (
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{
		Logger: logger.Discard,
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&Account{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestAuthServiceRefreshTokenLifecycle(t *testing.T) {
	db := newTestDB(t)
	account := &Account{
		Username: "alice",
		Role:     "admin",
	}
	if err := db.Create(account).Error; err != nil {
		t.Fatalf("create account: %v", err)
	}

	svc := NewAuthService(db, nil, "primary-secret")
	token, err := svc.IssueRefreshToken(account, time.Hour)
	if err != nil {
		t.Fatalf("issue refresh token: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty refresh token")
	}
	if account.RefreshTokenHash == "" || account.RefreshTokenExpiry == nil {
		t.Fatal("refresh token metadata not persisted")
	}

	found, err := svc.ValidateRefreshToken(token)
	if err != nil {
		t.Fatalf("validate refresh token: %v", err)
	}
	if found.Username != account.Username {
		t.Fatalf("expected username %s, got %s", account.Username, found.Username)
	}

	// Expired token is rejected
	expired := time.Now().Add(-time.Hour)
	account.RefreshTokenExpiry = &expired
	if err := db.Save(account).Error; err != nil {
		t.Fatalf("update expiry: %v", err)
	}
	if _, err := svc.ValidateRefreshToken(token); err == nil {
		t.Fatal("expected expired token error")
	}
}

func TestAuthServiceValidateRefreshTokenInvalid(t *testing.T) {
	db := newTestDB(t)
	svc := NewAuthService(db, nil, "primary-secret")
	if _, err := svc.ValidateRefreshToken("does-not-exist"); err == nil {
		t.Fatal("expected invalid token error")
	}
}
