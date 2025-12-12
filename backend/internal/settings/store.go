package settings

import (
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"

	"updockly/backend/internal/config"
	"updockly/backend/internal/vault"
)

// Record stores runtime settings in the database so they survive container restarts.
type Record struct {
	ID        uint                   `gorm:"primaryKey"`
	Data      config.RuntimeSettings `gorm:"type:jsonb;serializer:json"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Store struct {
	db    *gorm.DB
	vault *vault.Vault
}

func NewStore(db *gorm.DB, vault *vault.Vault) *Store {
	return &Store{db: db, vault: vault}
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

	data := rec.Data
	if s.vault != nil {
		if data.Notifications.DiscordToken != "" {
			if v, err := s.vault.Decrypt(data.Notifications.DiscordToken); err == nil {
				data.Notifications.DiscordToken = v
			}
		}
		if data.Notifications.SMTP.Password != "" {
			if v, err := s.vault.Decrypt(data.Notifications.SMTP.Password); err == nil {
				data.Notifications.SMTP.Password = v
			}
		}
		if data.SSO.ClientSecret != "" {
			if v, err := s.vault.Decrypt(data.SSO.ClientSecret); err == nil {
				data.SSO.ClientSecret = v
			}
		}
	}

	return data, true, nil
}

// Save upserts the provided settings and returns what was stored.
func (s *Store) Save(settings config.RuntimeSettings) (config.RuntimeSettings, error) {
	if s == nil || s.db == nil {
		return config.RuntimeSettings{}, errors.New("settings store not initialized")
	}

	stripped := stripEnvBackedFields(settings)
	if s.vault != nil {
		if stripped.Notifications.DiscordToken != "" {
			if v, err := s.vault.Encrypt(stripped.Notifications.DiscordToken); err == nil {
				stripped.Notifications.DiscordToken = v
			}
		}
		if stripped.Notifications.SMTP.Password != "" {
			if v, err := s.vault.Encrypt(stripped.Notifications.SMTP.Password); err == nil {
				stripped.Notifications.SMTP.Password = v
			}
		}
		if stripped.SSO.ClientSecret != "" {
			if v, err := s.vault.Encrypt(stripped.SSO.ClientSecret); err == nil {
				stripped.SSO.ClientSecret = v
			}
		}
	}

	rec := Record{
		ID:   1,
		Data: stripped,
	}
	if err := s.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{"data", "updated_at"}),
	}).Create(&rec).Error; err != nil {
		return config.RuntimeSettings{}, err
	}
	// Return the original settings (unencrypted) to the caller so they can use them immediately
	return stripEnvBackedFields(settings), nil
}

// stripEnvBackedFields removes values that should only be sourced from .env.
func stripEnvBackedFields(in config.RuntimeSettings) config.RuntimeSettings {
	in.DatabaseURL = ""
	in.ClientOrigin = ""
	return in
}
