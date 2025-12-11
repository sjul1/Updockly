package settings

import (
	"testing"

	"updockly/backend/internal/config"
)

func TestMergePrefersEnvBackedValues(t *testing.T) {
	base := config.RuntimeSettings{
		DatabaseURL:  "env-db",
		ClientOrigin: "env-origin",
		Timezone:     "UTC",
		HideSupport:  false,
		AutoPrune:    false,
	}
	stored := config.RuntimeSettings{
		DatabaseURL:  "stored-db",
		ClientOrigin: "stored-origin",
		Timezone:     "Europe/Paris",
		HideSupport:  true,
		AutoPrune:    true,
	}

	merged := Merge(base, stored)

	if merged.DatabaseURL != "env-db" || merged.ClientOrigin != "env-origin" {
		t.Fatalf("env-backed fields should remain from base: %+v", merged)
	}
	if merged.Timezone != "Europe/Paris" || !merged.HideSupport || !merged.AutoPrune {
		t.Fatalf("stored fields should overlay: %+v", merged)
	}
}

func TestNormalizeForStorageKeepsEnvDefaults(t *testing.T) {
	base := config.RuntimeSettings{
		DatabaseURL:  "env-db",
		ClientOrigin: "env-origin",
		Timezone:     "UTC",
		Notifications: config.NotificationSettings{
			SMTP: config.SMTPSettings{Port: 2525},
		},
	}
	incoming := config.RuntimeSettings{
		Timezone: "",
		Notifications: config.NotificationSettings{
			SMTP: config.SMTPSettings{},
		},
	}

	normalized := NormalizeForStorage(base, incoming)

	if normalized.DatabaseURL != "env-db" || normalized.ClientOrigin != "env-origin" {
		t.Fatalf("env-backed fields should remain from base: %+v", normalized)
	}
	if normalized.Timezone != "UTC" {
		t.Fatalf("timezone should fallback to base when empty, got %s", normalized.Timezone)
	}
	if normalized.Notifications.SMTP.Port != 2525 {
		t.Fatalf("SMTP port should inherit base default, got %d", normalized.Notifications.SMTP.Port)
	}
}
