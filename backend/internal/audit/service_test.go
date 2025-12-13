package audit

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"updockly/backend/internal/domain"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	if err := db.AutoMigrate(&domain.AuditLog{}); err != nil {
		t.Fatalf("failed to migrate database: %v", err)
	}
	return db
}

func TestService_Record(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db)

	err := svc.Record("user1", "alice", "login", "Successful login", "127.0.0.1")
	if err != nil {
		t.Fatalf("unexpected error recording audit log: %v", err)
	}

	var logs []domain.AuditLog
	if err := db.Find(&logs).Error; err != nil {
		t.Fatalf("failed to query logs: %v", err)
	}

	if len(logs) != 1 {
		t.Errorf("expected 1 log, got %d", len(logs))
	} else {
		log := logs[0]
		if log.UserID != "user1" {
			t.Errorf("expected UserID 'user1', got %q", log.UserID)
		}
		if log.UserName != "alice" {
			t.Errorf("expected UserName 'alice', got %q", log.UserName)
		}
		if log.Action != "login" {
			t.Errorf("expected Action 'login', got %q", log.Action)
		}
		if log.Details != "Successful login" {
			t.Errorf("expected Details 'Successful login', got %q", log.Details)
		}
		if log.IPAddress != "127.0.0.1" {
			t.Errorf("expected IPAddress '127.0.0.1', got %q", log.IPAddress)
		}
	}
}

func TestService_List(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db)

	// Insert dummy data
	_ = svc.Record("u1", "user1", "action1", "detail1", "ip1")
	_ = svc.Record("u2", "user2", "action2", "detail2", "ip2")
	_ = svc.Record("u3", "user3", "action3", "detail3", "ip3")

	t.Run("DefaultLimit", func(t *testing.T) {
		logs, err := svc.List(0)
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}
		if len(logs) != 3 {
			t.Errorf("expected 3 logs, got %d", len(logs))
		}
		// Expect reverse order (newest first)
		if logs[0].UserID != "u3" {
			t.Errorf("expected newest log first (u3), got %s", logs[0].UserID)
		}
	})

	t.Run("WithLimit", func(t *testing.T) {
		logs, err := svc.List(2)
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}
		if len(logs) != 2 {
			t.Errorf("expected 2 logs, got %d", len(logs))
		}
	})

	t.Run("LimitCap", func(t *testing.T) {
		// Should cap at 500
		// We only have 3, so we still get 3, but this verifies no error or crash
		logs, err := svc.List(1000)
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}
		if len(logs) != 3 {
			t.Errorf("expected 3 logs, got %d", len(logs))
		}
	})
}

func TestService_NilDB(t *testing.T) {
	svc := NewService(nil)

	// Record should fail silently (return nil error) as per current implementation fail-open
	if err := svc.Record("u", "n", "a", "d", "i"); err != nil {
		t.Errorf("expected nil error for nil db in Record, got %v", err)
	}

	// List should return error
	if _, err := svc.List(10); err == nil {
		t.Error("expected error for nil db in List, got nil")
	}
}
