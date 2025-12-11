package settings

import (
	"strings"

	"updockly/backend/internal/config"
)

// Merge overlays stored settings onto environment-backed defaults.
// Env-only values (DATABASE_URL, CLIENT_ORIGIN) are always taken from the base.
func Merge(base config.RuntimeSettings, stored config.RuntimeSettings) config.RuntimeSettings {
	merged := base
	merged.DatabaseURL = base.DatabaseURL
	merged.ClientOrigin = base.ClientOrigin

	if strings.TrimSpace(stored.SecretKey) != "" {
		merged.SecretKey = strings.TrimSpace(stored.SecretKey)
	}
	if stored.Timezone != "" {
		merged.Timezone = stored.Timezone
	}
	merged.HideSupport = stored.HideSupport
	merged.AutoPrune = stored.AutoPrune
	merged.Notifications = stored.Notifications
	merged.SSO = stored.SSO

	if merged.Timezone == "" {
		merged.Timezone = "UTC"
	}

	return merged
}

// NormalizeForStorage applies defaults and preserves env-only values before writing to the database.
func NormalizeForStorage(base config.RuntimeSettings, incoming config.RuntimeSettings) config.RuntimeSettings {
	normalized := base
	normalized.DatabaseURL = base.DatabaseURL
	normalized.ClientOrigin = base.ClientOrigin
	normalized.SecretKey = strings.TrimSpace(incoming.SecretKey)
	normalized.HideSupport = incoming.HideSupport
	normalized.AutoPrune = incoming.AutoPrune
	normalized.Timezone = incoming.Timezone
	if normalized.Timezone == "" {
		normalized.Timezone = base.Timezone
		if normalized.Timezone == "" {
			normalized.Timezone = "UTC"
		}
	}

	normalized.Notifications = incoming.Notifications
	if normalized.Notifications.SMTP.Port == 0 {
		normalized.Notifications.SMTP.Port = base.Notifications.SMTP.Port
		if normalized.Notifications.SMTP.Port == 0 {
			normalized.Notifications.SMTP.Port = 587
		}
	}

	normalized.SSO = incoming.SSO

	return normalized
}
