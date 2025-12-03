package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEscapeEnvValue(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple", "simple"},
		{"with space", "\"with space\""},
		{"with#hash", "\"with#hash\""},
		// These inputs are NOT already quoted in the way the function expects (wrapped in "..." or '...')
		// because they contain other characters or are just a string with quotes inside.
		// The function checks if the string *starts* and *ends* with matching quotes.
		{"already \"quoted\"", "'already \"quoted\"'"},
		{"already 'quoted'", "\"already 'quoted'\""},
		{"embedded \"quote\"", "'embedded \"quote\"'"},
		// Fully quoted strings should remain as is
		{"\"fully quoted\"", "\"fully quoted\""},
		{"'fully quoted'", "'fully quoted'"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := escapeEnvValue(tt.input); got != tt.expected {
				t.Errorf("escapeEnvValue(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestParseEnvValue(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple", "simple"},
		{"\"quoted\"", "quoted"},
		{"'single quoted'", "single quoted"},
		{"\"mixed 'quotes'\"", "mixed 'quotes'"},
		{"'mixed \"quotes\"'", "mixed \"quotes\""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := parseEnvValue(tt.input); got != tt.expected {
				t.Errorf("parseEnvValue(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestReadEnvFileMap(t *testing.T) {
	content := "\nKEY1=value1\nKEY2=\"value 2\"\n# Comment\nKEY3='value 3'\nexport KEY4=value4\n"
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, ".env")
	if err := os.WriteFile(tmpFile, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	m := readEnvFileMap(tmpFile)

	expected := map[string]string{
		"KEY1": "value1",
		"KEY2": "value 2",
		"KEY3": "value 3",
		"KEY4": "value4",
	}

	for k, v := range expected {
		if got, ok := m[k]; !ok || got != v {
			t.Errorf("expected key %q to be %q, got %q", k, v, got)
		}
	}
}

func TestCurrentRuntimeSettings(t *testing.T) {
	// Override EnvFilePath to avoid reading any local .env file
	originalEnvFile := EnvFilePath
	EnvFilePath = filepath.Join(t.TempDir(), "nonexistent.env")
	defer func() { EnvFilePath = originalEnvFile }()

	// Ensure clean state
	os.Clearenv()

	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/db")
	t.Setenv("CLIENT_ORIGIN", "http://example.com")
	t.Setenv("AUTO_PRUNE_IMAGES", "true")
	t.Setenv("SMTP_PORT", "25")
	t.Setenv("SMTP_ENABLED", "1")

	t.Logf("DATABASE_URL from env: %s", os.Getenv("DATABASE_URL"))

	settings := CurrentRuntimeSettings()

	if settings.DatabaseURL != "postgres://user:pass@localhost:5432/db" {
		t.Errorf("unexpected DatabaseURL: %q", settings.DatabaseURL)
	}
	if settings.ClientOrigin != "http://example.com" {
		t.Errorf("unexpected ClientOrigin: %q", settings.ClientOrigin)
	}
	if !settings.AutoPrune {
		t.Errorf("unexpected AutoPrune: %v", settings.AutoPrune)
	}
	if settings.Notifications.SMTP.Port != 25 {
		t.Errorf("unexpected SMTP Port: %d", settings.Notifications.SMTP.Port)
	}
	if !settings.Notifications.SMTP.Enabled {
		t.Errorf("unexpected SMTP Enabled: %v", settings.Notifications.SMTP.Enabled)
	}
}

func TestSaveRuntimeSettings(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, ".env")

	// Create initial file with some content to verify preservation
	initialContent := "SERVER_ADDR=:9090\nOLD_KEY=old_val\n"
	if err := os.WriteFile(tmpFile, []byte(initialContent), 0600); err != nil {
		t.Fatal(err)
	}

	settings := RuntimeSettings{
		DatabaseURL:  "sqlite://test.db",
		ClientOrigin: "http://localhost:3000",
		AutoPrune:    true,
		Notifications: NotificationSettings{
			SMTP: SMTPSettings{
				Port: 587,
			},
		},
	}

	if err := SaveRuntimeSettings(tmpFile, settings); err != nil {
		t.Fatalf("SaveRuntimeSettings failed: %v", err)
	}

	// Verify using readEnvFileMap which parses the file
	m := readEnvFileMap(tmpFile)

	checks := map[string]string{
		"DATABASE_URL":      "sqlite://test.db",
		"CLIENT_ORIGIN":     "http://localhost:3000",
		"AUTO_PRUNE_IMAGES": "true",
		"SERVER_ADDR":       ":9090",   // Preserved
		"OLD_KEY":           "old_val", // Preserved
	}

	for k, want := range checks {
		if got := m[k]; got != want {
			t.Errorf("key %s = %q, want %q", k, got, want)
		}
	}

	// Verify environment update (in current process)
	if os.Getenv("DATABASE_URL") != "sqlite://test.db" {
		t.Errorf("env var DATABASE_URL not updated to %s", "sqlite://test.db")
	}
}

func TestAtoiOrElse(t *testing.T) {
	if got := atoiOrElse("123", 0); got != 123 {
		t.Errorf("atoiOrElse(123) = %d", got)
	}
	if got := atoiOrElse("abc", 10); got != 10 {
		t.Errorf("atoiOrElse(abc) = %d", got)
	}
}
