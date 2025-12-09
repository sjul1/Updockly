package httpapi

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/mail"
	"net/smtp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type discordMessage struct {
	Content string `json:"content"`
}

func (s *Server) testNotificationHandler(c *gin.Context) {
	token := strings.TrimSpace(s.cfg.Notifications.DiscordToken)
	channel := strings.TrimSpace(s.cfg.Notifications.DiscordChannel)
	if token == "" || channel == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Discord bot token and channel are required"})
		return
	}

	if err := s.sendDiscordMessage(c.Request.Context(), "Test notification from Updockly"); err != nil {
		status := http.StatusBadGateway
		if errors.Is(err, errDiscordBadRequest) {
			status = http.StatusBadRequest
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Discord test notification sent"})
}

func (s *Server) testEmailHandler(c *gin.Context) {
	cfg := s.cfg.Notifications.SMTP
	if strings.TrimSpace(cfg.Host) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "SMTP not configured"})
		return
	}

	from := strings.TrimSpace(cfg.From)
	if from == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "SMTP_FROM is required"})
		return
	}

	// Require admin email for test delivery
	var admin Account
	if s.db == nil || s.db.Where("role = ?", "admin").First(&admin).Error != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Admin account not found to send test email"})
		return
	}
	target := strings.TrimSpace(admin.Email)
	if target == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Admin account has no email address configured"})
		return
	}

	if err := s.sendEmail([]string{target}, "Updockly Test Email", "This is a test email from Updockly."); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send email: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Test email sent to " + target})
}

func (s *Server) sendEmail(to []string, subject, body string) error {
	cfg := s.cfg.Notifications.SMTP
	if cfg.Host == "" {
		return errors.New("SMTP not configured")
	}

	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)

	envelopeFrom := strings.TrimSpace(cfg.From)
	headerFrom := envelopeFrom
	if parsed, err := mail.ParseAddress(cfg.From); err == nil {
		envelopeFrom = parsed.Address
		headerFrom = parsed.String()
	}

	// Setup authentication
	var auth smtp.Auth
	if cfg.User != "" && cfg.Password != "" {
		auth = smtp.PlainAuth("", cfg.User, cfg.Password, cfg.Host)
	}

	headerTo := make([]string, 0, len(to))
	rcptList := make([]string, 0, len(to))
	for _, r := range to {
		r = strings.TrimSpace(r)
		if parsed, err := mail.ParseAddress(r); err == nil {
			headerTo = append(headerTo, parsed.String())
			rcptList = append(rcptList, parsed.Address)
		} else if r != "" {
			headerTo = append(headerTo, r)
			rcptList = append(rcptList, r)
		}
	}
	if len(rcptList) == 0 {
		return fmt.Errorf("no valid recipients")
	}

	var msg bytes.Buffer
	fmt.Fprintf(&msg, "From: %s\r\n", headerFrom)
	fmt.Fprintf(&msg, "To: %s\r\n", strings.Join(headerTo, ","))
	fmt.Fprintf(&msg, "Subject: %s\r\n", subject)
	fmt.Fprintf(&msg, "\r\n%s\r\n", body)

	// If TLS is requested, establish a secure connection manually.
	if cfg.TLS {
		tlsConfig := &tls.Config{
			ServerName: cfg.Host,
		}
		conn, err := tls.Dial("tcp", addr, tlsConfig)
		if err != nil {
			return fmt.Errorf("tls dial: %w", err)
		}
		client, err := smtp.NewClient(conn, cfg.Host)
		if err != nil {
			return fmt.Errorf("create smtp client: %w", err)
		}
		defer client.Close()

		if auth != nil {
			if err := client.Auth(auth); err != nil {
				return fmt.Errorf("smtp auth: %w", err)
			}
		}
		if err := client.Mail(envelopeFrom); err != nil {
			return fmt.Errorf("smtp mail: %w", err)
		}
		for _, recipient := range rcptList {
			if err := client.Rcpt(recipient); err != nil {
				return fmt.Errorf("smtp rcpt: %w", err)
			}
		}
		writer, err := client.Data()
		if err != nil {
			return fmt.Errorf("smtp data: %w", err)
		}
		if _, err := writer.Write(msg.Bytes()); err != nil {
			return fmt.Errorf("smtp write: %w", err)
		}
		if err := writer.Close(); err != nil {
			return fmt.Errorf("smtp close: %w", err)
		}
		return client.Quit()
	}

	// Plain or STARTTLS-less connection
	return smtp.SendMail(addr, auth, envelopeFrom, rcptList, msg.Bytes())
}

var errDiscordBadRequest = fmt.Errorf("discord request invalid")

func (s *Server) sendDiscordMessage(ctx context.Context, content string) error {
	token := strings.TrimSpace(s.cfg.Notifications.DiscordToken)
	channel := strings.TrimSpace(s.cfg.Notifications.DiscordChannel)
	if token == "" || channel == "" {
		return fmt.Errorf("%w: discord token or channel missing", errDiscordBadRequest)
	}

	url := fmt.Sprintf("https://discord.com/api/v10/channels/%s/messages", channel)
	body, err := json.Marshal(discordMessage{Content: content})
	if err != nil {
		return fmt.Errorf("encode discord payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("build discord request: %w", err)
	}

	req.Header.Set("Authorization", "Bot "+token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("discord request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		msg := strings.TrimSpace(string(respBody))
		if msg == "" {
			msg = resp.Status
		}
		return fmt.Errorf("discord API error (%d): %s", resp.StatusCode, msg)
	}
	return nil
}

func (s *Server) sendWebhookMessage(ctx context.Context, content string) error {
	url := strings.TrimSpace(s.cfg.Notifications.WebhookURL)
	if url == "" {
		return nil
	}
	body, err := json.Marshal(discordMessage{Content: content})
	if err != nil {
		return fmt.Errorf("encode webhook payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("build webhook request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("webhook request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		msg := strings.TrimSpace(string(respBody))
		if msg == "" {
			msg = resp.Status
		}
		return fmt.Errorf("webhook error (%d): %s", resp.StatusCode, msg)
	}
	return nil
}

func (s *Server) sendImmediateNotification(entry UpdateHistory) {
	if s.cfg.Notifications.OnSuccess && entry.Status == "success" ||
		s.cfg.Notifications.OnFailure && entry.Status == "error" {
		loc := s.timezone
		if loc == nil {
			loc = time.Local
		}
		when := entry.CreatedAt.In(loc).Format("2006-01-02 15:04:05")
		name := entry.ContainerName
		if name == "" {
			name = entry.ContainerID
		}
		image := entry.Image
		if image == "" {
			image = "unknown image"
		}
		source := entry.Source
		if source != "" {
			// Best-effort capitalization without relying on deprecated strings.Title
			source = strings.ToUpper(source[:1]) + source[1:]
		}
		if entry.AgentName != "" {
			source = fmt.Sprintf("%s (%s)", source, entry.AgentName)
		}
		icon := "✅"
		if entry.Status != "success" {
			icon = "⚠️"
		}
		content := fmt.Sprintf(
			"%s Update %s\nContainer: %s\nImage: %s\nSource: %s\nStatus: %s\nWhen: %s\nMessage: %s",
			icon,
			entry.Status,
			name,
			image,
			source,
			entry.Status,
			when,
			entry.Message,
		)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = s.sendDiscordMessage(ctx, content)
		_ = s.sendWebhookMessage(ctx, content)
	}
}

func (s *Server) notifyAgentOffline(agent Agent) {
	if !s.cfg.Notifications.OnFailure {
		return
	}
	loc := s.timezone
	if loc == nil {
		loc = time.Local
	}
	last := "unknown"
	if agent.LastSeen != nil {
		last = agent.LastSeen.In(loc).Format("2006-01-02 15:04:05")
	}
	host := agent.Hostname
	if host == "" {
		host = "unknown host"
	}
	name := agent.Name
	if name == "" {
		name = agent.ID
	}
	content := fmt.Sprintf(
		"⚠️ Agent offline\nName: %s\nHost: %s\nLast seen: %s\nStatus: offline",
		name,
		host,
		last,
	)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = s.sendDiscordMessage(ctx, content)
	_ = s.sendWebhookMessage(ctx, content)
}

func (s *Server) SendPasswordResetEmail(to, token, origin string) error {
	link := fmt.Sprintf("%s/reset-password?token=%s", origin, token)
	subject := "Reset your Updockly password"
	body := fmt.Sprintf("Hello,\n\nYou requested a password reset. Click the link below to reset your password:\n\n%s\n\nIf you did not request this, please ignore this email.\n\nThis link expires in 1 hour.", link)
	return s.sendEmail([]string{to}, subject, body)
}
