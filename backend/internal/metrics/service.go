package metrics

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"updockly/backend/internal/domain"
)

type Service struct {
	db       *gorm.DB
	timezone *time.Location
}

func NewService(db *gorm.DB, tz *time.Location) *Service {
	return &Service{db: db, timezone: tz}
}

func (s *Service) RunningHistory(ctx context.Context) ([]domain.RunningSnapshot, error) {
	if s.db == nil {
		return nil, errors.New("database not ready")
	}

	s.ensureRunningSnapshot(ctx)

	var rows []domain.RunningSnapshot
	if err := s.db.Order("date DESC").Limit(7).Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *Service) ensureRunningSnapshot(ctx context.Context) {
	if s.db == nil {
		return
	}

	silentDB := s.db.Session(&gorm.Session{Logger: logger.Discard})

	if err := silentDB.AutoMigrate(&domain.RunningSnapshot{}); err != nil {
		return
	}

	loc := s.timezone
	if loc == nil {
		loc = time.Local
	}
	now := time.Now().In(loc)
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)

	var existing domain.RunningSnapshot
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
	snap := domain.RunningSnapshot{
		Date:    dayStart,
		Running: running,
		Total:   total,
	}
	_ = silentDB.Create(&snap).Error
}

func (s *Service) currentRunningCounts(ctx context.Context) (running int, total int) {
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
		var agents []domain.Agent
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
