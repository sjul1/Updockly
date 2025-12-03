package server

import (
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// Duplicate setupTestDB here if needed, or rely on handlers_test.go if it's compiled together.
// To be safe and self-contained, I'll define a helper here with a different name or reuse if I knew for sure.
// In Go, tests in the same package are compiled together. So setupTestDB from handlers_test.go is available.
// But for clarity, I will define `setupAuthTestDB`.

func setupAuthTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&Account{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestIssueAndVerifyToken(t *testing.T) {
	authService := NewAuthService(nil, nil, "test-secret")

	acc := Account{
		Username: "testuser",
		Role:     "admin",
		Name:     "Test User",
	}

	token, err := authService.IssueToken(acc, "session", time.Hour)
	if err != nil {
		t.Fatalf("issueToken failed: %v", err)
	}

	if token == "" {
		t.Fatal("issued token is empty")
	}

	claims, err := authService.VerifyToken(token)
	if err != nil {
		t.Fatalf("verifyToken failed: %v", err)
	}

	if claims.Subject != acc.Username {
		t.Errorf("expected subject %s, got %s", acc.Username, claims.Subject)
	}
	if claims.Role != acc.Role {
		t.Errorf("expected role %s, got %s", acc.Role, claims.Role)
	}
	if claims.Name != acc.Name {
		t.Errorf("expected name %s, got %s", acc.Name, claims.Name)
	}
	if claims.Type != "session" {
		t.Errorf("expected type session, got %s", claims.Type)
	}
}

func TestVerifyTokenExpired(t *testing.T) {
	authService := NewAuthService(nil, nil, "test-secret")

	acc := Account{Username: "expired"}
	token, err := authService.IssueToken(acc, "session", -1*time.Second)
	if err != nil {
		t.Fatalf("issueToken failed: %v", err)
	}

	_, err = authService.VerifyToken(token)
	if err == nil {
		t.Fatal("expected error for expired token, got nil")
	}
}

func TestVerifyTokenInvalidSignature(t *testing.T) {
	authService1 := NewAuthService(nil, nil, "secret-1")
	authService2 := NewAuthService(nil, nil, "secret-2")

	token, _ := authService1.IssueToken(Account{Username: "hacker"}, "session", time.Hour)

	_, err := authService2.VerifyToken(token)
	if err == nil {
		t.Fatal("expected error for invalid signature")
	}
}

func TestVerifyTokenMalformatted(t *testing.T) {
	authService := NewAuthService(nil, nil, "secret")
	_, err := authService.VerifyToken("not.a.jwt")
	if err == nil {
		t.Fatal("expected error for malformed token")
	}
}

func TestCreateAdminAndAuthenticate(t *testing.T) {
	db := setupAuthTestDB(t)
	vault := NewVault("test-secret-key-32-bytes-long-exactly!")
	authService := NewAuthService(db, vault, "jwt-secret")

	// Test CreateAdmin
	username := "admin"
	password := "securepass"
	email := "admin@example.com"
	totpSecret := "JBSWY3DPEHPK3PXP" // valid base32

	codes, err := authService.CreateAdmin(username, email, password, "Admin User", totpSecret)
	if err != nil {
		t.Fatalf("CreateAdmin failed: %v", err)
	}
	if len(codes) != 10 {
		t.Errorf("expected 10 recovery codes, got %d", len(codes))
	}

	// Test Authenticate Success
	acc, err := authService.Authenticate(username, password)
	if err != nil {
		t.Fatalf("Authenticate failed: %v", err)
	}
	if acc.Username != username {
		t.Errorf("Authenticate returned wrong user: %s", acc.Username)
	}

	// Test Authenticate Failure
	_, err = authService.Authenticate(username, "wrongpass")
	if err == nil {
		t.Error("expected error for wrong password")
	}

	_, err = authService.Authenticate("nonexistent", password)
	if err == nil {
		t.Error("expected error for nonexistent user")
	}
}

func TestAccountManagement(t *testing.T) {
	db := setupAuthTestDB(t)
	vault := NewVault("test-secret-key-32-bytes-long-exactly!")
	authService := NewAuthService(db, vault, "jwt-secret")

	_, err := authService.CreateAdmin("user1", "user1@test.com", "pass", "User One", "JBSWY3DPEHPK3PXP")
	if err != nil {
		t.Fatal(err)
	}

	// Test GetAccount
	acc, err := authService.GetAccount("user1")
	if err != nil {
		t.Fatalf("GetAccount failed: %v", err)
	}
	if acc.Email != "user1@test.com" {
		t.Errorf("got email %s, want user1@test.com", acc.Email)
	}

	// Test UpdateAccount
	updated, err := authService.UpdateAccount("user1", "New Name", "new@test.com", "", "")
	if err != nil {
		t.Fatalf("UpdateAccount failed: %v", err)
	}
	if updated.Name != "New Name" || updated.Email != "new@test.com" {
		t.Errorf("UpdateAccount not reflected")
	}

	// Test Password Change
	_, err = authService.UpdateAccount("user1", "", "", "wrong", "newpass")
	if err == nil || err.Error() != "invalid current password" {
		t.Error("expected invalid current password error")
	}

	_, err = authService.UpdateAccount("user1", "", "", "pass", "newpass")
	if err != nil {
		t.Fatalf("Change password failed: %v", err)
	}

	// Verify new password
	if _, err := authService.Authenticate("user1", "newpass"); err != nil {
		t.Error("failed to authenticate with new password")
	}
}

func Test2FAFlow(t *testing.T) {
	db := setupAuthTestDB(t)
	vault := NewVault("test-secret-key-32-bytes-long-exactly!")
	authService := NewAuthService(db, vault, "jwt-secret")

	// Create user without 2FA initially? CreateAdmin makes it enabled by default.
	// Let's manually create a user without 2FA
	userNo2FA := Account{
		Username:     "no2fa",
		PasswordHash: hashSecret("pass"),
	}
	db.Create(&userNo2FA)

	// Generate 2FA
	secret, _, err := authService.Generate2FA("no2fa")
	if err != nil {
		t.Fatalf("Generate2FA failed: %v", err)
	}

	// Enable 2FA
	passCode, _ := totp.GenerateCode(secret, time.Now())
	codes, err := authService.Enable2FA("no2fa", passCode)
	if err != nil {
		t.Fatalf("Enable2FA failed: %v", err)
	}
	if len(codes) == 0 {
		t.Error("expected recovery codes")
	}

	// Validate 2FA
	passCode, _ = totp.GenerateCode(secret, time.Now())
	valid, err := authService.Validate2FA("no2fa", passCode)
	if err != nil {
		t.Fatalf("Validate2FA failed: %v", err)
	}
	if !valid {
		t.Error("Validate2FA returned false")
	}

	// Validate Recovery Code
	recoveryCode := codes[0]
	valid, err = authService.Validate2FA("no2fa", recoveryCode)
	if err != nil {
		t.Fatalf("Validate2FA recovery failed: %v", err)
	}
	if !valid {
		t.Error("Validate2FA with recovery code returned false")
	}

	// Disable 2FA
	passCode, _ = totp.GenerateCode(secret, time.Now())
	err = authService.Disable2FA("no2fa", passCode, "pass")
	if err != nil {
		t.Fatalf("Disable2FA failed: %v", err)
	}

	acc, _ := authService.GetAccount("no2fa")
	if acc.TwoFactorEnabled {
		t.Error("2FA should be disabled")
	}
}
