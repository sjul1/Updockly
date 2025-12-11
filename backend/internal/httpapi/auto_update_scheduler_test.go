package httpapi

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupSchedulerTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&AgentCommand{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestWaitForAgentCommandsCompletes(t *testing.T) {
	db := setupSchedulerTestDB(t)
	cmd := AgentCommand{ID: "cmd-1", Status: "pending"}
	if err := db.Create(&cmd).Error; err != nil {
		t.Fatalf("seed command: %v", err)
	}

	srv := &Server{db: db}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go func() {
		time.Sleep(50 * time.Millisecond)
		_ = db.Model(&AgentCommand{}).Where("id = ?", cmd.ID).Update("status", "completed").Error
	}()

	if err := srv.waitForAgentCommands(ctx, []string{cmd.ID}, 1*time.Second); err != nil {
		t.Fatalf("expected wait to succeed, got %v", err)
	}
}

func TestWaitForAgentCommandsTimeout(t *testing.T) {
	db := setupSchedulerTestDB(t)
	cmd := AgentCommand{ID: "cmd-2", Status: "pending"}
	if err := db.Create(&cmd).Error; err != nil {
		t.Fatalf("seed command: %v", err)
	}

	srv := &Server{db: db}

	err := srv.waitForAgentCommands(context.Background(), []string{cmd.ID}, 100*time.Millisecond)
	if err == nil {
		t.Fatalf("expected timeout error")
	}
	if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) && !strings.Contains(err.Error(), "timeout") {
		t.Fatalf("unexpected error: %v", err)
	}
}
