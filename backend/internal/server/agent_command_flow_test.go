package server

import (
	"strings"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"updockly/backend/internal/config"
)

func newAgentCommandTestServer(t *testing.T) *Server {
	t.Helper()
	dsn := "file:" + strings.ReplaceAll(t.Name(), "/", "_") + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Discard,
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&Agent{}, &AgentCommand{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	cfg := config.Config{
		JWTSecret:    "jwt-key",
		VaultKey:     "vault-key",
		ClientOrigin: "http://localhost:8080",
	}
	srv, err := New(cfg, db)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	return srv
}

func TestCreateAgentCommandInternalCreatesRecord(t *testing.T) {
	srv := newAgentCommandTestServer(t)
	agent := &Agent{ID: "agent-1", Name: "agent"}
	if err := srv.db.Create(agent).Error; err != nil {
		t.Fatalf("create agent: %v", err)
	}

	cmd, err := srv.createAgentCommandInternal(agent.ID, "check-update", JSONMap{"containerId": "c1"})
	if err != nil {
		t.Fatalf("create command: %v", err)
	}
	if cmd.AgentID != agent.ID || cmd.Type != "check-update" {
		t.Fatalf("unexpected command: %+v", cmd)
	}

	var count int64
	if err := srv.db.Model(&AgentCommand{}).Count(&count).Error; err != nil {
		t.Fatalf("count commands: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 command, got %d", count)
	}
}

func TestCreateAgentCommandInternalMissingAgent(t *testing.T) {
	srv := newAgentCommandTestServer(t)
	if _, err := srv.createAgentCommandInternal("missing", "check-update", JSONMap{"containerId": "c1"}); err == nil {
		t.Fatal("expected error for missing agent")
	}
}
