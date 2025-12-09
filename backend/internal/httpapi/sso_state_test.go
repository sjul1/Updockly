package httpapi

import (
	"testing"

	"gorm.io/gorm"

	"updockly/backend/internal/config"
)

func TestSSOStateSignVerify(t *testing.T) {
	s := &Server{
		cfg: config.Config{
			ClientOrigin: "http://localhost",
		},
		jwtSecret: []byte("secret"),
		db:        &gorm.DB{}, // not used in these helpers
	}
	raw := "state-value"
	signed := raw + "." + s.signState(raw)
	if !s.verifyState(signed, signed) {
		t.Fatal("expected state verification to succeed")
	}
	if s.verifyState("tampered", signed) {
		t.Fatal("expected tampered state to fail verification")
	}
}
