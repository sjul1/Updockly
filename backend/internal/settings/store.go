package settings

import (
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"

	"updockly/backend/internal/config"
)

// Record stores runtime settings in the database so they survive container restarts.
type Record struct {
	ID        uint                   `gorm:"primaryKey"`
	Data      config.RuntimeSettings `gorm:"type:jsonb;serializer:json"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Store struct {
	db *gorm.DB
}

func NewStore(db *gorm.DB) *Store {
	return &Store{db: db}
}

// Load returns the stored settings. The boolean indicates whether a record was found.
func (s *Store) Load() (config.RuntimeSettings, bool, error) {
	if s == nil || s.db == nil {
		return config.RuntimeSettings{}, false, errors.New("settings store not initialized")
	}
	silent := s.db.Session(&gorm.Session{Logger: s.db.Logger.LogMode(logger.Silent)})
	var rec Record
	err := silent.First(&rec, 1).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return config.RuntimeSettings{}, false, nil
	}
	if err != nil {
		return config.RuntimeSettings{}, false, err
	}
	return rec.Data, true, nil
}

// Save upserts the provided settings and returns what was stored.
func (s *Store) Save(settings config.RuntimeSettings) (config.RuntimeSettings, error) {
	if s == nil || s.db == nil {
		return config.RuntimeSettings{}, errors.New("settings store not initialized")
	}
	rec := Record{
		ID:   1,
		Data: stripEnvBackedFields(settings),
	}
	if err := s.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{"data", "updated_at"}),
	}).Create(&rec).Error; err != nil {
		return config.RuntimeSettings{}, err
	}
	return rec.Data, nil
}

// stripEnvBackedFields removes values that should only be sourced from .env.
func stripEnvBackedFields(in config.RuntimeSettings) config.RuntimeSettings {
	in.DatabaseURL = ""
	in.ClientOrigin = ""
	return in
}
