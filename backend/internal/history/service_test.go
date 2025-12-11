package history

import (
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"updockly/backend/internal/domain"
)

func setupHistoryDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&domain.UpdateHistory{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestRecordAndList(t *testing.T) {
	db := setupHistoryDB(t)
	svc := NewService(db)

	// Record two entries
	first, err := svc.Record(domain.UpdateHistory{
		ContainerID:   "c1",
		ContainerName: "one",
		Image:         "img1",
		Source:        "local",
		Status:        "success",
		Message:       "ok",
	})
	if err != nil {
		t.Fatalf("record first: %v", err)
	}
	// Ensure different CreatedAt
	time.Sleep(5 * time.Millisecond)
	second, err := svc.Record(domain.UpdateHistory{
		ContainerID:   "c2",
		ContainerName: "two",
		Image:         "img2",
		Source:        "agent",
		Status:        "error",
		Message:       "fail",
	})
	if err != nil {
		t.Fatalf("record second: %v", err)
	}

	list, err := svc.List("10")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 history entries, got %d", len(list))
	}
	if list[0].ID != second.ID || list[1].ID != first.ID {
		t.Fatalf("expected most recent first, got %+v", list)
	}
}

func TestDelete(t *testing.T) {
	db := setupHistoryDB(t)
	svc := NewService(db)

	entry, err := svc.Record(domain.UpdateHistory{
		ContainerID: "c1",
		Status:      "success",
	})
	if err != nil {
		t.Fatalf("record: %v", err)
	}

	if err := svc.Delete(entry.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	// Ensure it was deleted
	list, err := svc.List("10")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("expected empty after delete, got %d", len(list))
	}
}

func TestListLimitsToDefault(t *testing.T) {
	db := setupHistoryDB(t)
	svc := NewService(db)

	for i := 0; i < 250; i++ {
		_, err := svc.Record(domain.UpdateHistory{
			ContainerID: "c",
			Status:      "success",
		})
		if err != nil {
			t.Fatalf("record %d: %v", i, err)
		}
	}

	list, err := svc.List("")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 200 {
		t.Fatalf("expected default limit 200, got %d", len(list))
	}
}
