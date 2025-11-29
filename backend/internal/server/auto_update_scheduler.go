package server

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// startAutoUpdateScheduler periodically checks cron schedules and triggers
// container updates for anything marked AutoUpdate=true (local and agents).
func (s *Server) startAutoUpdateScheduler(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// Track the last minute we ran per schedule to avoid double firing
	// when the ticker hits multiple times within the same minute.
	lastRun := make(map[string]time.Time)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.runAutoUpdateSchedules(ctx, lastRun)
		}
	}
}

type UpdateCycleStats struct {
	LocalChecked int
	LocalUpdated int
	LocalFailed  int
	AgentChecked int
	AgentQueued  int
}

func (s *Server) runAutoUpdateSchedules(ctx context.Context, lastRun map[string]time.Time) {
	if ctx != nil && ctx.Err() != nil {
		return
	}
	if s.db == nil {
		return
	}

	var schedules []Schedule
	if err := s.db.Find(&schedules).Error; err != nil {
		log.Printf("auto-update: failed to load schedules: %v", err)
		return
	}

	now := time.Now().In(s.timezone)
	for _, sched := range schedules {
		if !cronMatches(sched.CronExpression, now) {
			continue
		}
		if prev, ok := lastRun[sched.ID]; ok {
			if prev.Year() == now.Year() &&
				prev.Month() == now.Month() &&
				prev.Day() == now.Day() &&
				prev.Hour() == now.Hour() &&
				prev.Minute() == now.Minute() {
				continue
			}
		}
		lastRun[sched.ID] = now
		s.triggerAutoUpdateCycle(sched, now)
	}
}

func (s *Server) triggerAutoUpdateCycle(schedule Schedule, runAt time.Time) {
	if s.autoUpdateRun.Load() {
		return
	}
	if !s.autoUpdateRun.CompareAndSwap(false, true) {
		return
	}

	go func() {
		defer s.autoUpdateRun.Store(false)

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()

		if err := s.executeAutoUpdateCycle(ctx, schedule, runAt); err != nil {
			log.Printf("auto-update: schedule %s failed: %v", schedule.Name, err)
		}
	}()
}

func (s *Server) executeAutoUpdateCycle(ctx context.Context, schedule Schedule, runAt time.Time) error {
	if s.db == nil {
		return errors.New("database not ready")
	}
	if runAt.IsZero() {
		runAt = time.Now().In(s.timezone)
	} else {
		runAt = runAt.In(s.timezone)
	}

	log.Printf("auto-update: executing schedule %s at %s", schedule.Name, runAt.Format(time.RFC3339))

	stats := &UpdateCycleStats{}

	if err := s.updateLocalAutoUpdateContainers(ctx, stats); err != nil {
		log.Printf("auto-update: local update pass failed: %v", err)
	}

	if err := s.enqueueAgentAutoUpdates(ctx, stats); err != nil {
		log.Printf("auto-update: agent command enqueue failed: %v", err)
	}

	if s.cfg.AutoPruneImages {
		if err := s.pruneUnusedImages(ctx); err != nil {
			log.Printf("auto-update: image prune failed: %v", err)
		}
	}

	s.sendScheduleRecap(schedule.Name, stats)

	return nil
}

func (s *Server) sendScheduleRecap(scheduleName string, stats *UpdateCycleStats) {
	if stats.LocalChecked == 0 && stats.AgentChecked == 0 {
		return
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Executed schedule %s. ", scheduleName)
	fmt.Fprintf(&b, "Local: %d checked, %d updated, %d failed. ", stats.LocalChecked, stats.LocalUpdated, stats.LocalFailed)
	fmt.Fprintf(&b, "Agents: %d checked, %d queued.", stats.AgentChecked, stats.AgentQueued)

	s.recordUpdateHistory(UpdateHistory{
		Source:  "schedule",
		Status:  "info",
		Message: b.String(),
	})
}

func (s *Server) updateLocalAutoUpdateContainers(ctx context.Context, stats *UpdateCycleStats) error {
	if s.db == nil {
		return nil
	}

	silentDB := s.db.Session(&gorm.Session{Logger: logger.Discard})

	var settings []ContainerSettings
	if err := silentDB.Where("auto_update = ?", true).Find(&settings).Error; err != nil {
		return err
	}
	if len(settings) == 0 {
		return nil
	}

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return err
	}
	defer cli.Close()

	for _, cfg := range settings {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		if stats != nil {
			stats.LocalChecked++
		}

		origID := cfg.ID
		available, err := isUpdateAvailableWithClient(ctx, cli, cfg.ID)
		if err != nil && containerNotFound(err) {
			if newID, name, image := s.lookupContainerByNameOrImage(ctx, cli, cfg); newID != "" {
				// Check if the target ID already exists to avoid duplicates
				var count int64
				silentDB.Model(&ContainerSettings{}).Where("id = ?", newID).Count(&count)
				if count > 0 {
					log.Printf("auto-update: cleaning up stale record for %s (old: %s, new: %s)", cfg.Name, origID, newID)
					silentDB.Delete(&ContainerSettings{}, "id = ?", origID)
					continue
				}

				cfg.ID = newID
				if name != "" {
					cfg.Name = name
				}
				if image != "" {
					cfg.Image = image
				}
				_ = silentDB.Model(&ContainerSettings{}).Where("id = ?", origID).Updates(map[string]interface{}{
					"id":               cfg.ID,
					"name":             cfg.Name,
					"image":            cfg.Image,
					"update_available": false,
				}).Error
				available, err = isUpdateAvailableWithClient(ctx, cli, cfg.ID)
			}
		}
		if err != nil {
			if containerNotFound(err) {
				_ = silentDB.Model(&ContainerSettings{}).Where("id = ?", cfg.ID).
					Updates(map[string]interface{}{"auto_update": false, "update_available": false}).Error
			} else {
				log.Printf("auto-update: check failed for %s (%s): %v", cfg.Name, cfg.ID, err)
				_ = silentDB.Model(&ContainerSettings{}).Where("id = ?", cfg.ID).
					Update("update_available", false).Error
			}
			continue
		}

		_ = silentDB.Model(&ContainerSettings{}).Where("id = ?", cfg.ID).
			Update("update_available", available).Error

		if !available {
			continue
		}

		if _, name, image, digest, err := s.containerService.UpdateContainer(ctx, cfg.ID, func(m map[string]interface{}) {}); err != nil {
			if stats != nil {
				stats.LocalFailed++
			}
			s.recordUpdateHistory(UpdateHistory{
				ContainerID:   cfg.ID,
				ContainerName: cfg.Name,
				Image:         cfg.Image,
				Source:        "local",
				Status:        "error",
				Message:       fmt.Sprintf("Auto-update failed: %v", err),
			})
		} else {
			if stats != nil {
				stats.LocalUpdated++
			}
			// Success case needs history too?
			// The original code:
			// if err := s.updateContainerNoStream(ctx, cli, cfg.ID); err != nil { ... error history ... }
			// updateContainerNoStream recorded success history internally.
			// My new ContainerService.UpdateContainer DOES record DB changes but DOES NOT record history (I added DB sync but left history to caller).
			// So I need to record success history here.
			
			s.recordUpdateHistory(UpdateHistory{
				ContainerID:   cfg.ID,
				ContainerName: name,
				Image:         image,
				ImageDigest:   digest,
				Source:        "local",
				Status:        "success",
				Message:       fmt.Sprintf("Auto-updated container %s", name),
			})
		}
	}

	return nil
}

func (s *Server) pruneUnusedImages(ctx context.Context) error {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return err
	}
	defer cli.Close()

	report, err := cli.ImagesPrune(ctx, filters.Args{})
	if err != nil {
		return err
	}

	deleted := len(report.ImagesDeleted)
	reclaimedMB := float64(report.SpaceReclaimed) / 1_000_000
	log.Printf("auto-update: pruned %d images, reclaimed ~%.2f MB", deleted, reclaimedMB)
	return nil
}

func (s *Server) enqueueAgentAutoUpdates(ctx context.Context, stats *UpdateCycleStats) error {
	if s.db == nil {
		return nil
	}

	var agents []Agent
	if err := s.db.Find(&agents).Error; err != nil {
		return err
	}

	now := time.Now()
	for _, ag := range agents {
		if ag.LastSeen == nil || ag.LastSeen.Before(now.Add(-5*time.Minute)) {
			continue
		}

		containers := decodeContainers(ag)
		if len(containers) == 0 {
			continue
		}

		silent := s.db.Session(&gorm.Session{Logger: logger.Discard})
		var pending []AgentCommand
		_ = silent.Where("agent_id = ? AND status IN ?", ag.ID, []string{"pending", "running"}).Find(&pending).Error

		for _, cont := range containers {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if !cont.AutoUpdate || strings.TrimSpace(cont.ID) == "" {
				continue
			}
			
			if stats != nil {
				stats.AgentChecked++
			}

			if cont.UpdateAvailable {
				if s.hasAgentCommandForContainer(pending, cont.ID, "update-container") {
					continue
				}
				if _, err := s.createAgentCommandInternal(ag.ID, "update-container", JSONMap{"containerId": cont.ID}); err != nil {
					log.Printf("auto-update: queue update for agent %s/%s failed: %v", ag.Name, cont.ID, err)
				} else {
					if stats != nil {
						stats.AgentQueued++
					}
				}
				continue
			}

			if s.hasAgentCommandForContainer(pending, cont.ID, "check-update") {
				continue
			}
			if _, err := s.createAgentCommandInternal(ag.ID, "check-update", JSONMap{"containerId": cont.ID}); err != nil {
				log.Printf("auto-update: queue check for agent %s/%s failed: %v", ag.Name, cont.ID, err)
			}
		}
	}

	return nil
}

func (s *Server) hasAgentCommandForContainer(cmds []AgentCommand, containerID, cmdType string) bool {
	for _, cmd := range cmds {
		if cmd.Type != cmdType {
			continue
		}
		if cid, ok := cmd.Payload["containerId"].(string); ok && cid == containerID {
			return true
		}
	}
	return false
}

func containerNotFound(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "no such container") || strings.Contains(msg, "not found")
}

func cronMatches(expr string, t time.Time) bool {
	parts := strings.Fields(strings.TrimSpace(expr))
	if len(parts) != 5 {
		return false
	}

	minute := t.Minute()
	hour := t.Hour()
	day := t.Day()
	month := int(t.Month())
	weekday := int(t.Weekday()) // Sunday = 0

	matchers := []struct {
		field string
		value int
		min   int
		max   int
	}{
		{parts[0], minute, 0, 59},
		{parts[1], hour, 0, 23},
		{parts[2], day, 1, 31},
		{parts[3], month, 1, 12},
		{parts[4], weekday, 0, 6},
	}

	for _, m := range matchers {
		if !cronFieldMatches(m.field, m.value, m.min, m.max) {
			return false
		}
	}
	return true
}

func cronFieldMatches(field string, value, min, max int) bool {
	field = strings.TrimSpace(field)
	if field == "" {
		return false
	}

	parts := strings.Split(field, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if cronTokenMatches(part, value, min, max) {
			return true
		}
	}
	return false
}

func cronTokenMatches(token string, value, min, max int) bool {
	if token == "*" {
		return true
	}

	step := 1
	if strings.Contains(token, "/") {
		p := strings.SplitN(token, "/", 2)
		token = p[0]
		if parsed, err := strconv.Atoi(strings.TrimSpace(p[1])); err == nil && parsed > 0 {
			step = parsed
		} else {
			return false
		}
	}

	start, end := min, max
	switch {
	case token == "" || token == "*":
	case strings.Contains(token, "-"):
		p := strings.SplitN(token, "-", 2)
		if len(p) != 2 {
			return false
		}
		var err error
		start, err = strconv.Atoi(strings.TrimSpace(p[0]))
		if err != nil {
			return false
		}
		end, err = strconv.Atoi(strings.TrimSpace(p[1]))
		if err != nil {
			return false
		}
	default:
		v, err := strconv.Atoi(token)
		if err != nil {
			return false
		}
		if v == 7 && max == 6 {
			v = 0 // allow 0 or 7 for Sunday
		}
		return v == value
	}

	if start < min {
		start = min
	}
	if end > max {
		end = max
	}
	if start > end {
		return false
	}

	if value < start || value > end {
		return false
	}

	return (value-start)%step == 0
}

func (s *Server) lookupContainerByNameOrImage(ctx context.Context, cli *client.Client, cfg ContainerSettings) (id, name, image string) {
	containers, err := cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return "", "", ""
	}
	for _, cont := range containers {
		contName := ""
		if len(cont.Names) > 0 {
			contName = strings.TrimPrefix(cont.Names[0], "/")
		}
		if contName != "" && cfg.Name != "" && contName == cfg.Name {
			return cont.ID, contName, cont.Image
		}
		if cfg.Image != "" && cont.Image == cfg.Image {
			return cont.ID, contName, cont.Image
		}
	}
	return "", "", ""
}
