package config

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"math/big"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
)

// Config holds runtime configuration derived from environment variables.
type Config struct {
	Addr              string
	DatabaseURL       string
	SecretKey         string
	JWTSecret         string
	VaultKey          string
	JWTSecretPrevious string
	VaultKeyPrevious  string
	ClientOrigin      string
	LogLevel          string
	HideSupportButton bool
	Timezone          string
	AutoPruneImages   bool
	Notifications     NotificationSettings
	SSO               SSOSettings
	DBHost            string
	DBPort            int
	DBName            string
	AgentRequireIPBinding bool
}

// Load reads environment variables and applies sane defaults.
func Load() Config {
	loadEnvFile(EnvFilePath)
	settings := CurrentRuntimeSettings()
	dbURL := getEnv("DATABASE_URL", "")
	if dbURL == "" {
		dbURL = settings.DatabaseURL
	}

	rawSecret := strings.TrimSpace(getEnv("SECRET_KEY", ""))
	jwtSecret := strings.TrimSpace(getEnv("JWT_SECRET", ""))
	vaultKey := strings.TrimSpace(getEnv("VAULT_KEY", ""))

	// Migrate legacy SECRET_KEY to dedicated keys.
	if jwtSecret == "" && vaultKey == "" && rawSecret != "" {
		jwtSecret = rawSecret
		vaultKey = rawSecret
		_ = os.Setenv("JWT_SECRET", jwtSecret)
		_ = os.Setenv("VAULT_KEY", vaultKey)
		rawSecret = ""
	}

	// If still empty, generate secure defaults on first boot.
	if jwtSecret == "" {
		jwtSecret = randomString(48)
		_ = os.Setenv("JWT_SECRET", jwtSecret)
	}
	if vaultKey == "" {
		vaultKey = randomString(48)
		_ = os.Setenv("VAULT_KEY", vaultKey)
	}
	// Ensure distinct keys when both were missing and generated in the same run.
	if jwtSecret == vaultKey {
		vaultKey = randomString(48)
		_ = os.Setenv("VAULT_KEY", vaultKey)
	}

	cfg := Config{
		Addr:              getEnv("SERVER_ADDR", ":5000"),
		DatabaseURL:       dbURL,
		SecretKey:         rawSecret,
		JWTSecret:         jwtSecret,
		VaultKey:          vaultKey,
		JWTSecretPrevious: getEnv("JWT_SECRET_PREVIOUS", ""),
		VaultKeyPrevious:  getEnv("VAULT_KEY_PREVIOUS", ""),
		ClientOrigin:      os.Getenv("CLIENT_ORIGIN"),
		LogLevel:          getEnv("LOG_LEVEL", "info"),
		HideSupportButton: boolFromEnv("HIDE_SUPPORT_BUTTON"),
		AgentRequireIPBinding: boolFromEnv("AGENT_REQUIRE_IP_BINDING"),
		Timezone:          getEnv("TIMEZONE", "UTC"),
		AutoPruneImages:   settings.AutoPrune,
		Notifications:     settings.Notifications,
		SSO:               settings.SSO,
	}

	host, port, db := ParseDatabaseURL(cfg.DatabaseURL)
	cfg.DBHost = host
	cfg.DBPort = port
	cfg.DBName = db

	persistGeneratedSecrets(jwtSecret, vaultKey)

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

func randomString(length int) string {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	for i := range result {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		if err != nil {
			panic(err)
		}
		result[i] = chars[num.Int64()]
	}
	return string(result)
}

// persistGeneratedSecrets writes generated JWT/Vault secrets into the env file
// when they were not present, so the same keys are reused across restarts.
func persistGeneratedSecrets(jwtSecret, vaultKey string) {
	if jwtSecret == "" && vaultKey == "" {
		return
	}
	if _, err := os.Stat(EnvFilePath); err != nil {
		// Avoid creating new env files implicitly.
		return
	}
	env := readEnvFileMap(EnvFilePath)
	changed := false
	if jwtSecret != "" && env["JWT_SECRET"] == "" {
		env["JWT_SECRET"] = jwtSecret
		changed = true
	}
	if vaultKey != "" && env["VAULT_KEY"] == "" {
		env["VAULT_KEY"] = vaultKey
		changed = true
	}

	// If both dedicated secrets exist, remove legacy SECRET_KEY to avoid empty writes.
	if env["JWT_SECRET"] != "" && env["VAULT_KEY"] != "" {
		if _, ok := env["SECRET_KEY"]; ok {
			delete(env, "SECRET_KEY")
			changed = true
		}
	}

	if !changed {
		return
	}

	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var buf bytes.Buffer
	for _, k := range keys {
		fmt.Fprintf(&buf, "%s=%s\n", k, escapeEnvValue(env[k]))
	}
	_ = os.WriteFile(EnvFilePath, buf.Bytes(), 0o600)
}
