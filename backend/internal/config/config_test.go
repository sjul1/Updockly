package config

import (
	"os"
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
	if cfg.SecretKey != "dev-secret-key" {
		t.Errorf("expected default secret key, got %s", cfg.SecretKey)
	}
	if cfg.Timezone != "UTC" {
		t.Errorf("expected default timezone UTC, got %s", cfg.Timezone)
	}
}
