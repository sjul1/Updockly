package server

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func (s *Server) runningHistoryHandler(c *gin.Context) {
	if s.db == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "database not ready"})
		return
	}

	ctx := c.Request.Context()
	s.ensureRunningSnapshot(ctx)

	var rows []RunningSnapshot
	if err := s.db.Order("date DESC").Limit(7).Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load running history"})
		return
	}

	c.JSON(http.StatusOK, rows)
}

func (s *Server) ensureRunningSnapshot(ctx context.Context) {
	if s.db == nil {
		return
	}

	silentDB := s.db.Session(&gorm.Session{Logger: logger.Discard})

	if err := silentDB.AutoMigrate(&RunningSnapshot{}); err != nil {
		return
	}

	loc := s.timezone
	if loc == nil {
		loc = time.Local
	}
	now := time.Now().In(loc)
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)

	var existing RunningSnapshot
	err := silentDB.Where("date = ?", dayStart).First(&existing).Error
	if err == nil {
		running, total := s.currentRunningCounts(ctx)
		_ = silentDB.Model(&existing).Updates(map[string]interface{}{
			"running": running,
			"total":   total,
		}).Error
		return
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) && !strings.Contains(strings.ToLower(err.Error()), "does not exist") {
		return
	}

	running, total := s.currentRunningCounts(ctx)
	snap := RunningSnapshot{
		Date:    dayStart,
		Running: running,
		Total:   total,
	}
	_ = silentDB.Create(&snap).Error
}

func (s *Server) currentRunningCounts(ctx context.Context) (running int, total int) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err == nil {
		defer cli.Close()
		if containers, err := cli.ContainerList(ctx, container.ListOptions{All: true}); err == nil {
			for _, cont := range containers {
				state := strings.ToLower(cont.State)
				if state == "running" {
					running++
				}
				total++
			}
		}
	}

	if s.db != nil {
		var agents []Agent
		if err := s.db.Find(&agents).Error; err == nil {
			cutoff := time.Now().Add(-5 * time.Minute)
			for _, ag := range agents {
				if ag.LastSeen == nil || ag.LastSeen.Before(cutoff) {
					continue
				}
				for _, cont := range ag.Containers {
					state := strings.ToLower(cont.State)
					if state == "running" {
						running++
					}
					total++
				}
			}
		}
	}

	return running, total
}
