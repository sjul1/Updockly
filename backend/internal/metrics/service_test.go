package metrics

import (
	"context"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"updockly/backend/internal/domain"
)

func setupMetricsDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&domain.RunningSnapshot{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestRecordRunningSnapshot(t *testing.T) {
	db := setupMetricsDB(t)
	svc := NewService(db, time.UTC)

	svc.ensureRunningSnapshot(context.Background())

	var rows []domain.RunningSnapshot
	if err := db.Find(&rows).Error; err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 snapshot, got %d", len(rows))
	}
	if rows[0].Running < 0 || rows[0].Total < 0 {
		t.Fatalf("unexpected snapshot values: %+v", rows[0])
	}
}
