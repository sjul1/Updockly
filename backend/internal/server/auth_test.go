package server

import (
	"testing"
	"time"
)

func TestIssueAndVerifyToken(t *testing.T) {
	// Mock DB not needed for token issue/verify in AuthService
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
	// Issue token that expired 1 second ago
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