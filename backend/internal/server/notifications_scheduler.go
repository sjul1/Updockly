package server

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

func (s *Server) startNotificationScheduler(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	s.checkOfflineAgents()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.maybeSendRecap()
			s.checkOfflineAgents()
		}
	}
}

func (s *Server) maybeSendRecap() {
	if s.db == nil {
		return
	}

	if strings.TrimSpace(s.cfg.Notifications.DiscordToken) == "" &&
		strings.TrimSpace(s.cfg.Notifications.WebhookURL) == "" &&
		!s.cfg.Notifications.SMTP.Enabled {
		return
	}

	recapTime := strings.TrimSpace(s.cfg.Notifications.RecapTime)
	if recapTime == "" {
		return
	}

	hour, min, ok := parseClockTime(recapTime)
	if !ok {
		return
	}

	now := time.Now().In(s.timezone)
	target := time.Date(now.Year(), now.Month(), now.Day(), hour, min, 0, 0, s.timezone)

	// Prevent an immediate recap on first boot when the server starts after the scheduled time.
	if !s.recapPrimed {
		s.recapPrimed = true
		if now.After(target) {
			s.lastRecapDate = target.Format("2006-01-02")
			return
		}
	}

	sentToday := s.lastRecapDate == target.Format("2006-01-02")
	if sentToday || now.Before(target) {
		return
	}

	if err := s.sendRecap(now.Add(-24*time.Hour), now); err != nil {
		fmt.Printf("recap: failed to send: %v\n", err)
		return
	}

	s.lastRecapDate = target.Format("2006-01-02")
}

func (s *Server) checkOfflineAgents() {
	if s.db == nil {
		return
	}

	// Grace period: don't send offline notifications immediately on startup
	if time.Since(s.startedAt) < 2*time.Minute {
		return
	}

	var agents []Agent
	if err := s.db.Find(&agents).Error; err != nil {
		return
	}
	cutoff := time.Now().Add(-5 * time.Minute)
	for _, ag := range agents {
		// If LastSeen is nil, the agent has never connected, so it's not "offline" in the sense of being down.
		if ag.LastSeen == nil {
			continue
		}
		if ag.LastSeen.Before(cutoff) {
			if s.markAgentOfflineNotified(ag.ID) {
				s.notifyAgentOffline(ag)
			}
		} else {
			s.clearAgentOfflineNotified(ag.ID)
		}
	}
}

func parseClockTime(value string) (int, int, bool) {
	parts := strings.Split(value, ":")
	if len(parts) != 2 {
		return 0, 0, false
	}
	var h, m int
	_, errH := fmt.Sscanf(parts[0], "%d", &h)
	_, errM := fmt.Sscanf(parts[1], "%d", &m)
	if errH != nil || errM != nil || h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, 0, false
	}
	return h, m, true
}

func (s *Server) sendRecap(since, until time.Time) error {
	if s.db == nil {
		return nil
	}

	var rows []UpdateHistory
	if err := s.db.Where("created_at BETWEEN ? AND ?", since, until).Order("created_at DESC").Find(&rows).Error; err != nil {
		return fmt.Errorf("load update history: %w", err)
	}

	loc := s.timezone
	if loc == nil {
		loc = time.Local
	}
	now := until.In(loc)
	var b strings.Builder
	fmt.Fprintf(&b, "📦 Updockly recap — %s (%s)\n", now.Format("2006-01-02 15:04"), loc.String())

	if len(rows) == 0 {
		b.WriteString("No activity in the last 24h.\n")
	} else {
		successCount := 0
		failureCount := 0
		for _, row := range rows {
			switch row.Status {
			case "success":
				successCount++
			case "error":
				failureCount++
			}
		}
		fmt.Fprintf(&b, "Success: %d | Failed: %d | Total Entries: %d\n", successCount, failureCount, len(rows))
		limit := 20
		if len(rows) < limit {
			limit = len(rows)
		}
		for i := 0; i < limit; i++ {
			row := rows[i]
			when := row.CreatedAt.In(loc).Format("15:04")
			
			if row.Source == "schedule" && row.Status == "info" {
				fmt.Fprintf(&b, "📅 %s @ %s\n   %s\n", "Schedule Run", when, row.Message)
				continue
			}

			source := strings.Title(row.Source)
			if row.AgentName != "" {
				source = fmt.Sprintf("%s (%s)", source, row.AgentName)
			}
			name := row.ContainerName
			if name == "" {
				name = row.ContainerID
			}
			image := row.Image
			if image == "" {
				image = "unknown image"
			}
			icon := "✅"
			if row.Status != "success" {
				icon = "⚠️"
			}
			fmt.Fprintf(&b, "%s %s — %s via %s @ %s [%s]\n", icon, name, image, source, when, row.Status)
		}
		if len(rows) > limit {
			fmt.Fprintf(&b, "…and %d more\n", len(rows)-limit)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := s.sendDiscordMessage(ctx, b.String()); err != nil && !errors.Is(err, errDiscordBadRequest) {
		return err
	}
	if err := s.sendWebhookMessage(ctx, b.String()); err != nil {
		return err
	}

	// Send Email Recap
	if s.cfg.Notifications.SMTP.Enabled {
		var admins []Account
		if err := s.db.Where("role = ? AND email != ''", "admin").Find(&admins).Error; err == nil && len(admins) > 0 {
			var recipients []string
			for _, admin := range admins {
				recipients = append(recipients, admin.Email)
			}
			subject := fmt.Sprintf("Updockly Daily Recap - %s", now.Format("2006-01-02"))
			// Use the same body as other notifications for now, or format it nicely as HTML?
			// Plain text is safer for now.
			if err := s.sendEmail(recipients, subject, b.String()); err != nil {
				fmt.Printf("recap: failed to send email: %v\n", err)
				// Don't return error to avoid blocking other notifications if they succeeded?
				// But the function signature returns error.
				// Let's just log it.
			}
		}
	}

	return nil
}

