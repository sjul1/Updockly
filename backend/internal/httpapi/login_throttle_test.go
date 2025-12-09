package httpapi

import (
	"log/slog"
	"testing"
	"time"
)

func TestLoginThrottleBlocksAfterFailures(t *testing.T) {
	s := &Server{
		loginAttempts: make(map[string]loginAttempt),
		log:           slog.Default(),
	}

	key := "user|ip"
	for i := 0; i < 5; i++ {
		s.recordLoginFailure(key)
	}
	delay, blocked := s.isLoginBlocked(key)
	if !blocked {
		t.Fatal("expected login to be blocked after 5 failures")
	}
	if delay <= 0 {
		t.Fatalf("expected positive retry delay, got %v", delay)
	}
}

func TestLoginThrottleResetsAfterIdle(t *testing.T) {
	s := &Server{
		loginAttempts: make(map[string]loginAttempt),
		log:           slog.Default(),
	}
	key := "user|ip"
	past := time.Now().Add(-31 * time.Minute)
	s.loginAttempts[key] = loginAttempt{count: 4, lastAttempt: past}

	if delay, blocked := s.isLoginBlocked(key); blocked {
		t.Fatalf("expected not blocked, got delay %v", delay)
	}
	if _, ok := s.loginAttempts[key]; ok {
		t.Fatal("expected stale login attempt to be cleared")
	}
}
