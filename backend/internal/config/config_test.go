package config

import (
	"os"
	"path/filepath"
	"testing"
)

func splitEnv(s string) []string {
	for i := 0; i < len(s); i++ {
		if s[i] == '=' {
			return []string{s[:i], s[i+1:]}
		}
	}
	return []string{s, ""}
}

func TestParseDatabaseURL(t *testing.T) {
	tests := []struct {
		name     string
		dsn      string
		wantHost string
		wantPort int
		wantDB   string
	}{
		{
			name:     "Standard Postgres",
			dsn:      "postgres://user:pass@localhost:5432/dbname?sslmode=disable",
			wantHost: "localhost",
			wantPort: 5432,
			wantDB:   "dbname",
		},
		{
			name:     "Postgres No Port",
			dsn:      "postgres://user:pass@dbhost/prod_db",
			wantHost: "dbhost",
			wantPort: 5432,
			wantDB:   "prod_db",
		},
		{
			name:     "IPv4 Address",
			dsn:      "postgres://user:pass@192.168.1.50:5432/app",
			wantHost: "192.168.1.50",
			wantPort: 5432,
			wantDB:   "app",
		},
		{
			name:     "SQLite path",
			dsn:      "sqlite://data.db",
			wantHost: "data.db",
			wantPort: 5432, // Implementation defaults to 5432
			wantDB:   "",
		},
		{
			name:     "Empty",
			dsn:      "",
			wantHost: "",
			wantPort: 5432, // Implementation defaults to 5432
			wantDB:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, port, db := ParseDatabaseURL(tt.dsn)
			if host != tt.wantHost {
				t.Errorf("got host %q, want %q", host, tt.wantHost)
			}
			if port != tt.wantPort {
				t.Errorf("got port %d, want %d", port, tt.wantPort)
			}
			if db != tt.wantDB {
				t.Errorf("got db %q, want %q", db, tt.wantDB)
			}
		})
	}
}

func TestLoadDefaults(t *testing.T) {
	// Save current env to restore later
	originalEnv := os.Environ()
	defer func() {
		os.Clearenv()
		for _, pair := range originalEnv {
			// rudimentary restore
			parts := splitEnv(pair)
			if len(parts) == 2 {
				os.Setenv(parts[0], parts[1])
			}
		}
	}()

	// Unset relevant keys
	os.Unsetenv("SERVER_ADDR")
	os.Unsetenv("DATABASE_URL")
	os.Unsetenv("SECRET_KEY")
	os.Unsetenv("TIMEZONE")

	cfg := Load()

	if cfg.Addr != ":5000" {
		t.Errorf("expected default addr :5000, got %s", cfg.Addr)
	}
	if cfg.SecretKey != "" {
		t.Errorf("expected empty secret key when unset, got %s", cfg.SecretKey)
	}
	if cfg.JWTSecret == "" {
		t.Fatalf("expected generated jwt secret when unset")
	}
	if cfg.VaultKey == "" {
		t.Fatalf("expected generated vault key when unset")
	}
	if cfg.JWTSecret == cfg.VaultKey {
		t.Fatalf("expected distinct generated secrets")
	}
	if cfg.Timezone != "UTC" {
		t.Errorf("expected default timezone UTC, got %s", cfg.Timezone)
	}
}

func TestLoadSecretMigratesToJWTAndVault(t *testing.T) {
	origEnvFile := EnvFilePath
	EnvFilePath = filepath.Join(t.TempDir(), "config.env")
	defer func() { EnvFilePath = origEnvFile }()

	os.Clearenv()
	os.Setenv("SECRET_KEY", "primary-secret")

	cfg := Load()

	if cfg.JWTSecret != "primary-secret" {
		t.Fatalf("expected JWT secret to mirror SECRET_KEY, got %s", cfg.JWTSecret)
	}
	if cfg.VaultKey == "" {
		t.Fatalf("expected Vault key to be set after migration")
	}
	if cfg.VaultKey == cfg.JWTSecret {
		t.Fatalf("expected Vault key to rotate away from SECRET_KEY when both match")
	}
	if cfg.SecretKey != "" {
		t.Fatalf("expected legacy SECRET_KEY to be cleared after migration, got %s", cfg.SecretKey)
	}
}

func TestLoadGeneratesWhenUnset(t *testing.T) {
	os.Clearenv()

	cfg := Load()

	if cfg.JWTSecret == "" || cfg.VaultKey == "" {
		t.Fatalf("expected generated jwt/vault secrets when none provided")
	}
	if cfg.JWTSecret == cfg.VaultKey {
		t.Fatalf("expected distinct generated secrets for jwt and vault")
	}
}
