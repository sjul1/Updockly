package audit

import (
	"errors"

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

func (s *Service) List(limit int) ([]domain.AuditLog, error) {
	if s.db == nil {
		return nil, errors.New("database not ready")
	}
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}

	var logs []domain.AuditLog
	if err := s.db.Order("created_at DESC").Limit(limit).Find(&logs).Error; err != nil {
		return nil, err
	}
	return logs, nil
}

func (s *Service) Record(userID, username, action, details, ip string) error {
	if s.db == nil {
		return nil // Fail open if DB not ready? Or return error. For logging, fail open is often safer for availability.
	}

	log := domain.AuditLog{
		UserID:    userID,
		UserName:  username,
		Action:    action,
		Details:   details,
		IPAddress: ip,
	}

	// Use silent logger to avoid noise for every action
	return s.db.Session(&gorm.Session{Logger: logger.Discard}).Create(&log).Error
}
