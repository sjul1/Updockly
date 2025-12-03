package config

import (
	"bufio"
	"bytes"
	"fmt"
	"net/mail"
	"os"
	"strconv"
	"strings"
)

const EnvFilePath = ".env"

type NotificationSettings struct {
	WebhookURL       string       `json:"webhookUrl"`
	DiscordToken     string       `json:"discordToken"`
	DiscordChannel   string       `json:"discordChannel"`
	OnSuccess        bool         `json:"onSuccess"`
	OnFailure        bool         `json:"onFailure"`
	RecapTime        string       `json:"recapTime"`
	NotificationCron string       `json:"notificationCron"`
	SMTP             SMTPSettings `json:"smtp"`
}

type SMTPSettings struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	From     string `json:"from"`
	TLS      bool   `json:"tls"`
	Enabled  bool   `json:"enabled"`
}

type SSOSettings struct {
	Enabled      bool   `json:"enabled"`
	Provider     string `json:"provider"` // e.g., "authentik"
	IssuerURL    string `json:"issuerUrl"`
	ClientID     string `json:"clientId"`
	ClientSecret string `json:"clientSecret"`
	RedirectURL  string `json:"redirectUrl"`
}

type RuntimeSettings struct {
	DatabaseURL   string               `json:"databaseUrl"`
	ClientOrigin  string               `json:"clientOrigin"`
	SecretKey     string               `json:"secretKey"`
	Timezone      string               `json:"timezone"`
	AutoPrune     bool                 `json:"autoPruneImages"`
	Notifications NotificationSettings `json:"notifications"`
	SSO           SSOSettings          `json:"sso"`
}

func loadEnvFile(path string) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		if strings.HasPrefix(key, "export ") {
			key = strings.TrimSpace(strings.TrimPrefix(key, "export "))
		}
		value := parseEnvValue(strings.TrimSpace(parts[1]))
		_ = os.Setenv(key, value)
	}
}

func parseEnvValue(value string) string {
	if len(value) >= 2 {
		if (strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"")) ||
			(strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'")) {
			return value[1 : len(value)-1]
		}
	}
	return value
}

func getEnvWithFile(key string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	if file := os.Getenv(key + "_FILE"); file != "" {
		if content, err := os.ReadFile(file); err == nil {
			return strings.TrimSpace(string(content))
		}
	}
	return ""
}

func boolFromEnv(key string) bool {
	val := strings.ToLower(getEnvWithFile(key))
	return val == "true" || val == "1" || val == "yes"
}

func CurrentRuntimeSettings() RuntimeSettings {
	loadEnvFile(EnvFilePath)
	cleanAddress := func(val string) string {
		parsed, err := mail.ParseAddress(val)
		if err == nil && parsed != nil {
			return parsed.String()
		}
		return strings.TrimSpace(strings.ReplaceAll(val, "\\\"", "\""))
	}
	return RuntimeSettings{
		DatabaseURL:  getEnvWithFile("DATABASE_URL"),
		ClientOrigin: func() string {
			if co := getEnvWithFile("CLIENT_ORIGIN"); co != "" {
				return co
			}
			return "http://localhost:8080"
		}(),
		SecretKey:    getEnvWithFile("SECRET_KEY"),
		Timezone:     getEnvWithFile("TIMEZONE"),
		AutoPrune:    boolFromEnv("AUTO_PRUNE_IMAGES"),
		Notifications: NotificationSettings{
			WebhookURL:       getEnvWithFile("NOTIFICATION_WEBHOOK_URL"),
			DiscordToken:     getEnvWithFile("NOTIFICATION_DISCORD_TOKEN"),
			DiscordChannel:   getEnvWithFile("NOTIFICATION_DISCORD_CHANNEL"),
			OnSuccess:        boolFromEnv("NOTIFICATION_ON_SUCCESS"),
			OnFailure:        boolFromEnv("NOTIFICATION_ON_FAILURE"),
			RecapTime:        getEnvWithFile("NOTIFICATION_RECAP_TIME"),
			NotificationCron: getEnvWithFile("NOTIFICATION_CRON"),
			SMTP: SMTPSettings{
				Host:     getEnvWithFile("SMTP_HOST"),
				Port:     atoiOrElse(getEnvWithFile("SMTP_PORT"), 587),
				User:     getEnvWithFile("SMTP_USER"),
				Password: getEnvWithFile("SMTP_PASSWORD"),
				From:     cleanAddress(getEnvWithFile("SMTP_FROM")),
				TLS:      boolFromEnv("SMTP_TLS"),
				Enabled:  boolFromEnv("SMTP_ENABLED"),
			},
		},
		SSO: SSOSettings{
			Enabled:      boolFromEnv("SSO_ENABLED"),
			Provider:     getEnvWithFile("SSO_PROVIDER"),
			IssuerURL:    getEnvWithFile("SSO_ISSUER_URL"),
			ClientID:     getEnvWithFile("SSO_CLIENT_ID"),
			ClientSecret: getEnvWithFile("SSO_CLIENT_SECRET"),
			RedirectURL:  getEnvWithFile("SSO_REDIRECT_URL"),
		},
	}
}

func SaveRuntimeSettings(path string, settings RuntimeSettings) error {
	existing := readEnvFileMap(path)
	// If SERVER_ADDR (or other preserved keys) exist only in environment, keep them.
	preservedEnvKeys := []string{"SERVER_ADDR"}
	for _, k := range preservedEnvKeys {
		if _, ok := existing[k]; !ok {
			if val := os.Getenv(k); val != "" {
				existing[k] = val
			}
		}
	}

	var buf bytes.Buffer
	write := func(key, value string) {
		if value == "" {
			fmt.Fprintf(&buf, "%s=\n", key)
			return
		}
		fmt.Fprintf(&buf, "%s=%s\n", key, escapeEnvValue(value))
	}

	write("DATABASE_URL", settings.DatabaseURL)
	write("CLIENT_ORIGIN", settings.ClientOrigin)
	write("SECRET_KEY", settings.SecretKey)
	write("TIMEZONE", settings.Timezone)
	write("AUTO_PRUNE_IMAGES", strconv.FormatBool(settings.AutoPrune))
	write("NOTIFICATION_WEBHOOK_URL", settings.Notifications.WebhookURL)
	write("NOTIFICATION_DISCORD_TOKEN", settings.Notifications.DiscordToken)
	write("NOTIFICATION_DISCORD_CHANNEL", settings.Notifications.DiscordChannel)
	write("NOTIFICATION_ON_SUCCESS", strconv.FormatBool(settings.Notifications.OnSuccess))
	write("NOTIFICATION_ON_FAILURE", strconv.FormatBool(settings.Notifications.OnFailure))
	write("NOTIFICATION_RECAP_TIME", settings.Notifications.RecapTime)
	write("NOTIFICATION_CRON", settings.Notifications.NotificationCron)
	write("SMTP_HOST", settings.Notifications.SMTP.Host)
	write("SMTP_PORT", strconv.Itoa(settings.Notifications.SMTP.Port))
	write("SMTP_USER", settings.Notifications.SMTP.User)
	write("SMTP_PASSWORD", settings.Notifications.SMTP.Password)
	write("SMTP_FROM", settings.Notifications.SMTP.From)
	write("SMTP_TLS", strconv.FormatBool(settings.Notifications.SMTP.TLS))
	write("SMTP_ENABLED", strconv.FormatBool(settings.Notifications.SMTP.Enabled))
	write("SSO_ENABLED", strconv.FormatBool(settings.SSO.Enabled))
	write("SSO_PROVIDER", settings.SSO.Provider)
	write("SSO_ISSUER_URL", settings.SSO.IssuerURL)
	write("SSO_CLIENT_ID", settings.SSO.ClientID)
	write("SSO_CLIENT_SECRET", settings.SSO.ClientSecret)
	write("SSO_REDIRECT_URL", settings.SSO.RedirectURL)

	// Preserve SERVER_ADDR if present, even though it's not part of settings.
	if addr, ok := existing["SERVER_ADDR"]; ok && addr != "" {
		write("SERVER_ADDR", addr)
	}

	// Preserve any extra keys that we don't manage directly (e.g., SERVER_ADDR).
	known := map[string]struct{}{
		"DATABASE_URL":                 {},
		"CLIENT_ORIGIN":                {},
		"SECRET_KEY":                   {},
		"TIMEZONE":                     {},
		"SERVER_ADDR":                  {},
		"AUTO_PRUNE_IMAGES":            {},
		"NOTIFICATION_WEBHOOK_URL":     {},
		"NOTIFICATION_DISCORD_TOKEN":   {},
		"NOTIFICATION_DISCORD_CHANNEL": {},
		"NOTIFICATION_ON_SUCCESS":      {},
		"NOTIFICATION_ON_FAILURE":      {},
		"NOTIFICATION_RECAP_TIME":      {},
		"NOTIFICATION_CRON":            {},
		"SMTP_HOST":                    {},
		"SMTP_PORT":                    {},
		"SMTP_USER":                    {},
		"SMTP_PASSWORD":                {},
		"SMTP_FROM":                    {},
		"SMTP_TLS":                     {},
		"SMTP_ENABLED":                 {},
		"SSO_ENABLED":                  {},
		"SSO_PROVIDER":                 {},
		"SSO_ISSUER_URL":               {},
		"SSO_CLIENT_ID":                {},
		"SSO_CLIENT_SECRET":            {},
		"SSO_REDIRECT_URL":             {},
	}
	for k, v := range existing {
		if _, ok := known[k]; ok {
			continue
		}
		write(k, v)
	}

	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		return err
	}

	// push values into environment for current process
	for key, value := range map[string]string{
		"DATABASE_URL":                 settings.DatabaseURL,
		"CLIENT_ORIGIN":                settings.ClientOrigin,
		"SECRET_KEY":                   settings.SecretKey,
		"TIMEZONE":                     settings.Timezone,
		"AUTO_PRUNE_IMAGES":            strconv.FormatBool(settings.AutoPrune),
		"NOTIFICATION_WEBHOOK_URL":     settings.Notifications.WebhookURL,
		"NOTIFICATION_DISCORD_TOKEN":   settings.Notifications.DiscordToken,
		"NOTIFICATION_DISCORD_CHANNEL": settings.Notifications.DiscordChannel,
		"NOTIFICATION_ON_SUCCESS":      strconv.FormatBool(settings.Notifications.OnSuccess),
		"NOTIFICATION_ON_FAILURE":      strconv.FormatBool(settings.Notifications.OnFailure),
		"NOTIFICATION_RECAP_TIME":      settings.Notifications.RecapTime,
		"NOTIFICATION_CRON":            settings.Notifications.NotificationCron,
		"SSO_ENABLED":                  strconv.FormatBool(settings.SSO.Enabled),
		"SSO_PROVIDER":                 settings.SSO.Provider,
		"SSO_ISSUER_URL":               settings.SSO.IssuerURL,
		"SSO_CLIENT_ID":                settings.SSO.ClientID,
		"SSO_CLIENT_SECRET":            settings.SSO.ClientSecret,
		"SSO_REDIRECT_URL":             settings.SSO.RedirectURL,
	} {
		_ = os.Setenv(key, value)
	}

	return nil
}

func readEnvFileMap(path string) map[string]string {
	out := make(map[string]string)
	data, err := os.ReadFile(path)
	if err != nil {
		return out
	}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		if strings.HasPrefix(key, "export ") {
			key = strings.TrimSpace(strings.TrimPrefix(key, "export "))
		}
		if key == "" {
			continue
		}
		out[key] = parseEnvValue(strings.TrimSpace(parts[1]))
	}
	return out
}

func escapeEnvValue(value string) string {
	trimmed := strings.TrimSpace(value)
	// If already wrapped in quotes, keep as-is to avoid double-escaping.
	if len(trimmed) >= 2 && ((trimmed[0] == '"' && trimmed[len(trimmed)-1] == '"') || (trimmed[0] == '\'' && trimmed[len(trimmed)-1] == '\'')) {
		return value
	}
	if strings.ContainsAny(value, " #") {
		// If the value already contains double-quotes (e.g. display name), wrap with single quotes instead of escaping quotes.
		if strings.Contains(value, "\"") {
			return "'" + value + "'"
		}
		return strconv.Quote(value)
	}
	return value
}

func atoiOrElse(value string, fallback int) int {
	if v, err := strconv.Atoi(value); err == nil {
		return v
	}
	return fallback
}
