package history

import (
	"errors"
	"strconv"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"updockly/backend/internal/domain"
)

type Service struct {
	db *gorm.DB
}

func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

func (s *Service) List(limitParam string) ([]domain.UpdateHistory, error) {
	if s.db == nil {
		return nil, errors.New("database not ready")
	}

	limit := 200
	if raw := strings.TrimSpace(limitParam); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 && parsed <= 500 {
			limit = parsed
		}
	}

	rows := []domain.UpdateHistory{}
	if err := s.db.Order("created_at DESC").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *Service) Delete(id string) error {
	if s.db == nil {
		return errors.New("database not ready")
	}

	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("missing history id")
	}

	result := s.db.Delete(&domain.UpdateHistory{}, "id = ?", id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (s *Service) Record(entry domain.UpdateHistory) (domain.UpdateHistory, error) {
	if s.db == nil {
		return entry, nil
	}

	entry.Source = strings.TrimSpace(strings.ToLower(entry.Source))
	if entry.Source == "" {
		entry.Source = "local"
	}
	entry.Status = strings.TrimSpace(strings.ToLower(entry.Status))
	if entry.Status == "" {
		entry.Status = "success"
	}
	entry.Message = strings.TrimSpace(entry.Message)

	silentDB := s.db.Session(&gorm.Session{Logger: logger.Discard})
	err := silentDB.Create(&entry).Error
	return entry, err
}
