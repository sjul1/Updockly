package domain

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// StringList stores a slice as JSON in the database.
type StringList []string

func (s StringList) Value() (driver.Value, error) {
	if len(s) == 0 {
		return "[]", nil
	}
	b, err := json.Marshal(s)
	return string(b), err
}

func (s *StringList) Scan(value interface{}) error {
	if value == nil {
		*s = []string{}
		return nil
	}
	switch v := value.(type) {
	case []byte:
		return json.Unmarshal(v, s)
	case string:
		return json.Unmarshal([]byte(v), s)
	default:
		return errors.New("unsupported type for StringList")
	}
}

// ContainerSnapshotList stores container snapshots as JSON in the database.
type ContainerSnapshotList []ContainerSnapshot

func (c ContainerSnapshotList) Value() (driver.Value, error) {
	if len(c) == 0 {
		return "[]", nil
	}
	b, err := json.Marshal(c)
	return string(b), err
}

func (c *ContainerSnapshotList) Scan(value interface{}) error {
	if value == nil {
		*c = []ContainerSnapshot{}
		return nil
	}
	switch v := value.(type) {
	case []byte:
		return json.Unmarshal(v, c)
	case string:
		return json.Unmarshal([]byte(v), c)
	default:
		return errors.New("unsupported type for ContainerSnapshotList")
	}
}

type ContainerSnapshot struct {
	ID              string     `json:"id"`
	Name            string     `json:"name"`
	Image           string     `json:"image"`
	Status          string     `json:"status"`
	State           string     `json:"state"`
	AutoUpdate      bool       `json:"autoUpdate"`
	UpdateAvailable bool       `json:"updateAvailable"`
	CheckedAt       *time.Time `json:"checkedAt,omitempty"`
	Ports           []string   `json:"ports,omitempty"`
	Labels          []string   `json:"labels,omitempty"`
}

// JSONMap persists arbitrary JSON documents.
type JSONMap map[string]interface{}

func (m JSONMap) Value() (driver.Value, error) {
	if len(m) == 0 {
		return "{}", nil
	}
	b, err := json.Marshal(m)
	return string(b), err
}

func (m *JSONMap) Scan(value interface{}) error {
	if value == nil {
		*m = JSONMap{}
		return nil
	}
	switch v := value.(type) {
	case []byte:
		return json.Unmarshal(v, m)
	case string:
		return json.Unmarshal([]byte(v), m)
	default:
		return errors.New("unsupported type for JSONMap")
	}
}

type Account struct {
	ID                 string `gorm:"primaryKey"`
	Name               string
	Username           string `gorm:"uniqueIndex"`
	Email              string
	PasswordHash       string
	ResetToken         string
	ResetTokenHash     string
	ResetTokenExpiry   *time.Time
	RefreshTokenHash   string
	RefreshTokenExpiry *time.Time
	Role               string
	TwoFactorSecret    string
	TwoFactorEnabled   bool
	RecoveryCodes      StringList `gorm:"type:jsonb"`
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

func (a *Account) BeforeCreate(*gorm.DB) error {
	if a.ID == "" {
		a.ID = uuid.NewString()
	}
	return nil
}

type ContainerSettings struct {
	ID              string `gorm:"primaryKey"`
	Name            string
	Image           string
	AutoUpdate      bool
	UpdateAvailable bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (c *ContainerSettings) BeforeCreate(*gorm.DB) error {
	if c.ID == "" {
		c.ID = uuid.NewString()
	}
	return nil
}

type UpdateHistory struct {
	ID            string    `gorm:"primaryKey" json:"id"`
	ContainerID   string    `json:"containerId"`
	ContainerName string    `json:"containerName"`
	Image         string    `json:"image"`
	ImageDigest   string    `json:"imageDigest,omitempty"`
	AgentID       string    `json:"agentId,omitempty"`
	AgentName     string    `json:"agentName,omitempty"`
	Source        string    `json:"source"`
	Status        string    `json:"status"`
	Message       string    `json:"message"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

func (h *UpdateHistory) BeforeCreate(*gorm.DB) error {
	if h.ID == "" {
		h.ID = uuid.NewString()
	}
	return nil
}

type RunningSnapshot struct {
	ID        string    `gorm:"primaryKey" json:"id"`
	Date      time.Time `gorm:"index" json:"date"`
	Running   int       `json:"running"`
	Total     int       `json:"total"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (r *RunningSnapshot) BeforeCreate(*gorm.DB) error {
	if r.ID == "" {
		r.ID = uuid.NewString()
	}
	return nil
}

type Schedule struct {
	ID             string `gorm:"primaryKey"`
	Name           string
	CronExpression string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (s *Schedule) BeforeCreate(*gorm.DB) error {
	if s.ID == "" {
		s.ID = uuid.NewString()
	}
	return nil
}

type Agent struct {
	ID             string                `gorm:"primaryKey" json:"id"`
	Name           string                `json:"name"`
	Hostname       string                `json:"hostname"`
	AgentVersion   string                `json:"agentVersion"`
	DockerVersion  string                `json:"dockerVersion"`
	Platform       string                `json:"platform"`
	Notes          string                `json:"notes"`
	Token          string                `json:"-"` // legacy stored secret for agent auth
	TokenHash      string                `json:"-"`
	TokenVersion   int                   `json:"-"`
	TokenExpiresAt *time.Time            `json:"-"`
	TokenBinding   string                `json:"-"`
	LastSeen       *time.Time            `json:"lastSeen,omitempty"`
	Containers     ContainerSnapshotList `gorm:"type:jsonb;serializer:json" json:"-"`
	TLSEnabled     bool                  `json:"tlsEnabled"`
	CPU            float64               `json:"cpu"`
	Memory         float64               `json:"memory"`
	CreatedAt      time.Time             `json:"createdAt"`
	UpdatedAt      time.Time             `json:"updatedAt"`
}

func (a *Agent) BeforeCreate(*gorm.DB) error {
	if a.ID == "" {
		a.ID = uuid.NewString()
	}
	return nil
}

type AgentCommand struct {
	ID          string     `gorm:"primaryKey" json:"id"`
	AgentID     string     `gorm:"index" json:"agentId"`
	Type        string     `json:"type"`
	Status      string     `json:"status"`
	Payload     JSONMap    `gorm:"type:jsonb;serializer:json" json:"payload,omitempty"`
	Result      JSONMap    `gorm:"type:jsonb;serializer:json" json:"result,omitempty"`
	Error       string     `json:"error,omitempty"`
	StartedAt   *time.Time `json:"startedAt,omitempty"`
	CompletedAt *time.Time `json:"completedAt,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}

func (c *AgentCommand) BeforeCreate(*gorm.DB) error {
	if c.ID == "" {
		c.ID = uuid.NewString()
	}
	if c.Status == "" {
		c.Status = "pending"
	}
	return nil
}

type AuditLog struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    string    `json:"userId"`
	UserName  string    `json:"username"`
	Action    string    `json:"action"`
	Details   string    `json:"details"`
	IPAddress string    `json:"ipAddress"`
	CreatedAt time.Time `json:"createdAt"`
}
