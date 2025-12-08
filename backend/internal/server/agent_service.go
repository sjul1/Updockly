package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type AgentService struct {
	db               *gorm.DB
	requireIPBinding bool
}

const agentTokenTTL = 30 * 24 * time.Hour

func NewAgentService(db *gorm.DB, requireIPBinding bool) *AgentService {
	return &AgentService{db: db, requireIPBinding: requireIPBinding}
}

func (s *AgentService) Create(name, hostname, notes string, tlsEnabled bool) (*Agent, error) {
	token := RandomString(48)
	hash := sha256.Sum256([]byte(token))
	exp := time.Now().Add(agentTokenTTL)
	agent := &Agent{
		Name:           name,
		Hostname:       hostname,
		Notes:          notes,
		TokenHash:      hex.EncodeToString(hash[:]),
		TokenVersion:   1,
		TokenExpiresAt: &exp,
		TLSEnabled:     tlsEnabled,
	}
	if err := s.db.Create(agent).Error; err != nil {
		return nil, err
	}
	agent.Token = token // return plain token without persisting it in DB
	return agent, nil
}

func (s *AgentService) List() ([]Agent, error) {
	var agents []Agent
	if err := s.db.Order("created_at DESC").Find(&agents).Error; err != nil {
		return nil, err
	}
	return agents, nil
}

func (s *AgentService) Get(id string) (*Agent, error) {
	var agent Agent
	if err := s.db.Where("id = ?", id).First(&agent).Error; err != nil {
		return nil, err
	}
	return &agent, nil
}

func (s *AgentService) GetByToken(token, ip string) (*Agent, error) {
	if token == "" {
		return nil, gorm.ErrRecordNotFound
	}
	if s.requireIPBinding && strings.TrimSpace(ip) == "" {
		return nil, errors.New("agent token requires IP binding")
	}
	hash := sha256.Sum256([]byte(token))
	hashHex := hex.EncodeToString(hash[:])
	var agent Agent
	silentDB := s.db.Session(&gorm.Session{Logger: logger.Discard})

	err := silentDB.Where("token_hash = ?", hashHex).First(&agent).Error
	if err != nil {
		// Legacy fallback to plain token
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if err := silentDB.Where("token = ?", token).First(&agent).Error; err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	// Expiry check
	if agent.TokenExpiresAt != nil && time.Now().After(*agent.TokenExpiresAt) {
		return nil, errors.New("agent token expired")
	}

	// Optional IP binding: if stored, require match. If empty, bind on first successful auth.
	if agent.TokenBinding != "" && ip != "" {
		if agent.TokenBinding != ip {
			return nil, errors.New("agent token not valid for this IP")
		}
	} else if ip != "" && agent.TokenBinding == "" {
		_ = silentDB.Model(&Agent{}).Where("id = ?", agent.ID).Update("token_binding", ip)
		agent.TokenBinding = ip
	}

	return &agent, nil
}

func (s *AgentService) Update(id, name, hostname, notes string, tlsEnabled bool) (*Agent, error) {
	agent, err := s.Get(id)
	if err != nil {
		return nil, err
	}
	agent.Name = name
	agent.Hostname = hostname
	agent.Notes = notes
	agent.TLSEnabled = tlsEnabled
	if err := s.db.Save(agent).Error; err != nil {
		return nil, err
	}
	return agent, nil
}

func (s *AgentService) Delete(id string) error {
	return s.db.Delete(&Agent{}, "id = ?", id).Error
}

func (s *AgentService) RotateToken(id string) (*Agent, error) {
	agent, err := s.Get(id)
	if err != nil {
		return nil, err
	}
	token := RandomString(48)
	hash := sha256.Sum256([]byte(token))
	exp := time.Now().Add(agentTokenTTL)
	agent.TokenHash = hex.EncodeToString(hash[:])
	agent.TokenVersion++
	agent.TokenExpiresAt = &exp
	if err := s.db.Save(agent).Error; err != nil {
		return nil, err
	}
	agent.Token = token // return plain token without persisting it
	return agent, nil
}

func (s *AgentService) CreateCommand(agentID, cmdType string, payload JSONMap) (*AgentCommand, error) {
	var agent Agent
	if err := s.db.Where("id = ?", agentID).First(&agent).Error; err != nil {
		return nil, err
	}
	cmd := AgentCommand{
		AgentID: agent.ID,
		Type:    cmdType,
		Status:  "pending",
		Payload: payload,
	}
	if err := s.db.Create(&cmd).Error; err != nil {
		return nil, err
	}
	return &cmd, nil
}

func (s *AgentService) GetNextCommand(agentID string) (*AgentCommand, error) {
	var cmd AgentCommand
	err := s.db.Session(&gorm.Session{Logger: logger.Discard}).
		Where("agent_id = ? AND status = ?", agentID, "pending").
		Order("created_at ASC").
		First(&cmd).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &cmd, nil
}

func (s *AgentService) UpdateCommand(cmd *AgentCommand) error {
	return s.db.Save(cmd).Error
}

func (s *AgentService) UpdateCommandWithContext(ctx context.Context, cmd *AgentCommand) error {
	return s.db.WithContext(ctx).Save(cmd).Error
}

func (s *AgentService) GetCommand(id, agentID string) (*AgentCommand, error) {
	var cmd AgentCommand
	if err := s.db.Where("id = ? AND agent_id = ?", id, agentID).First(&cmd).Error; err != nil {
		return nil, err
	}
	return &cmd, nil
}

func (s *AgentService) ToggleContainerAutoUpdate(agentID, containerID string, enabled bool) error {
	agent, err := s.Get(agentID)
	if err != nil {
		return err
	}

	containers := decodeContainers(*agent)
	found := false
	for i := range containers {
		if containers[i].ID == containerID {
			containers[i].AutoUpdate = enabled
			found = true
			break
		}
	}
	if !found {
		containers = append(containers, ContainerSnapshot{
			ID:         containerID,
			AutoUpdate: enabled,
		})
	}
	agent.Containers = containers
	return s.db.Save(agent).Error
}

func containerDetailsFromReport(agent *Agent, result JSONMap, cmdPayload JSONMap) (string, string, string) {
	containerID := ""
	name := ""
	image := ""

	if v, ok := result["containerId"].(string); ok {
		containerID = v
	}
	if containerID == "" && cmdPayload != nil {
		if v, ok := cmdPayload["containerId"].(string); ok {
			containerID = v
		}
	}

	if v, ok := result["container"].(map[string]interface{}); ok {
		if n, ok := v["name"].(string); ok {
			name = n
		}
		if i, ok := v["image"].(string); ok {
			image = i
		}
	}

	if (name == "" || image == "") && containerID != "" {
		// Try to find name/image from agent snapshot
		for _, c := range decodeContainers(*agent) {
			if c.ID == containerID {
				if name == "" {
					name = c.Name
				}
				if image == "" {
					image = c.Image
				}
				break
			}
		}
	}

	return containerID, name, image
}
