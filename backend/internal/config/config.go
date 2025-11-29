package config

import (
	"net/url"
	"os"
	"strconv"
	"strings"
)

// Config holds runtime configuration derived from environment variables.
type Config struct {
	Addr                 string
	DatabaseURL          string
	SecretKey            string
	ClientOrigin         string
	Timezone             string
	AutoPruneImages      bool
	Notifications        NotificationSettings
	SSO                  SSOSettings
	DBHost               string
	DBPort               int
	DBName               string
}

// Load reads environment variables and applies sane defaults.
func Load() Config {
	if _, err := os.Stat(EnvFilePath); os.IsNotExist(err) {
		settings := CurrentRuntimeSettings()
		_ = SaveRuntimeSettings(EnvFilePath, settings)
	}

	loadEnvFile(EnvFilePath)
	settings := CurrentRuntimeSettings()
	cfg := Config{
		Addr:                 getEnv("SERVER_ADDR", ":5000"),
		DatabaseURL:          getEnv("DATABASE_URL", "postgres://updockly:updockly@localhost:5432/updocklydb?sslmode=disable"),
		SecretKey:            getEnv("SECRET_KEY", "dev-secret-key"),
		ClientOrigin:         os.Getenv("CLIENT_ORIGIN"),
		Timezone:             getEnv("TIMEZONE", "UTC"),
		AutoPruneImages:      settings.AutoPrune,
		Notifications:        settings.Notifications,
		SSO:                  settings.SSO,
	}

	host, port, db := ParseDatabaseURL(cfg.DatabaseURL)
	cfg.DBHost = host
	cfg.DBPort = port
	cfg.DBName = db

	return cfg
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	if file := os.Getenv(key + "_FILE"); file != "" {
		if content, err := os.ReadFile(file); err == nil {
			return strings.TrimSpace(string(content))
		}
	}
	return fallback
}

func ParseDatabaseURL(dsn string) (string, int, string) {
	u, err := url.Parse(dsn)
	if err != nil {
		return "", 0, ""
	}
	host := u.Hostname()
	if host == "" && u.Host != "" {
		host = u.Host
	}
	port := 0
	if p := u.Port(); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil {
			port = parsed
		}
	}
	if port == 0 {
		port = 5432
	}
	db := strings.TrimPrefix(u.Path, "/")
	if db == "" {
		if parts := strings.Split(u.Opaque, "@"); len(parts) == 2 {
			if after := strings.Split(parts[1], "/"); len(after) == 2 {
				db = after[1]
			}
		}
	}
	return host, port, db
}
