package server

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupAgentTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&Agent{}, &AgentCommand{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestAgentCRUD(t *testing.T) {
	db := setupAgentTestDB(t)
	svc := NewAgentService(db)

	// Create
	agent, err := svc.Create("test-agent", "host1", "notes", true)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if agent.Name != "test-agent" {
		t.Errorf("Name = %s, want test-agent", agent.Name)
	}
	if agent.Token == "" {
		t.Error("Token is empty")
	}

	// Get
	got, err := svc.Get(agent.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.Hostname != "host1" {
		t.Errorf("Hostname = %s, want host1", got.Hostname)
	}

	// List
	list, err := svc.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("List returned %d, want 1", len(list))
	}

	// Update
	updated, err := svc.Update(agent.ID, "new-name", "host2", "new notes", false)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if updated.Name != "new-name" || !updated.CreatedAt.Equal(agent.CreatedAt) {
		t.Error("Update failed to persist or return correct data")
	}

	// GetByToken
	byToken, err := svc.GetByToken(agent.Token)
	if err != nil {
		t.Fatalf("GetByToken failed: %v", err)
	}
	if byToken.ID != agent.ID {
		t.Error("GetByToken returned wrong agent")
	}

	// RotateToken
	oldToken := agent.Token
	rotated, err := svc.RotateToken(agent.ID)
	if err != nil {
		t.Fatalf("RotateToken failed: %v", err)
	}
	if rotated.Token == oldToken {
		t.Error("Token did not change")
	}

	// Delete
	if err := svc.Delete(agent.ID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	_, err = svc.Get(agent.ID)
	if err == nil {
		t.Error("Get should fail after delete")
	}
}

func TestAgentCommands(t *testing.T) {
	db := setupAgentTestDB(t)
	svc := NewAgentService(db)

	agent, _ := svc.Create("agent", "host", "", false)

	// CreateCommand
	payload := JSONMap{"foo": "bar"}
	cmd, err := svc.CreateCommand(agent.ID, "test-cmd", payload)
	if err != nil {
		t.Fatalf("CreateCommand failed: %v", err)
	}
	if cmd.Status != "pending" {
		t.Errorf("Status = %s, want pending", cmd.Status)
	}

	// GetNextCommand
	next, err := svc.GetNextCommand(agent.ID)
	if err != nil {
		t.Fatalf("GetNextCommand failed: %v", err)
	}
	if next == nil {
		t.Fatal("GetNextCommand returned nil")
	}
	if next.ID != cmd.ID {
		t.Error("GetNextCommand returned wrong command")
	}

	// UpdateCommand (mark as running)
	next.Status = "running"
	if err := svc.UpdateCommand(next); err != nil {
		t.Fatalf("UpdateCommand failed: %v", err)
	}

	// GetNextCommand (should be nil now)
	next2, err := svc.GetNextCommand(agent.ID)
	if err != nil {
		t.Fatal(err)
	}
	if next2 != nil {
		t.Error("GetNextCommand returned a command, expected nil (none pending)")
	}

	// GetCommand
	fetched, err := svc.GetCommand(cmd.ID, agent.ID)
	if err != nil {
		t.Fatalf("GetCommand failed: %v", err)
	}
	if fetched.Status != "running" {
		t.Error("GetCommand returned stale data")
	}
}

func TestToggleContainerAutoUpdate_Agent(t *testing.T) {
	db := setupAgentTestDB(t)
	svc := NewAgentService(db)

	agent, _ := svc.Create("agent", "host", "", false)

	// Toggle on new container
	err := svc.ToggleContainerAutoUpdate(agent.ID, "cont1", true)
	if err != nil {
		t.Fatal(err)
	}

	refetched, _ := svc.Get(agent.ID)
	containers := decodeContainers(*refetched)
	if len(containers) != 1 {
		t.Fatalf("expected 1 container, got %d", len(containers))
	}
	if containers[0].ID != "cont1" || !containers[0].AutoUpdate {
		t.Error("container not updated correctly")
	}

	// Toggle off existing
	err = svc.ToggleContainerAutoUpdate(agent.ID, "cont1", false)
	if err != nil {
		t.Fatal(err)
	}
	refetched, _ = svc.Get(agent.ID)
	containers = decodeContainers(*refetched)
	if containers[0].AutoUpdate {
		t.Error("AutoUpdate should be false")
	}
}
