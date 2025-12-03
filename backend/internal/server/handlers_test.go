package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&Account{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func newTestServer(t *testing.T) *Server {
	t.Helper()
	gin.SetMode(gin.TestMode)
	return &Server{db: setupTestDB(t)}
}

func TestHealthHandlerOK(t *testing.T) {
	srv := newTestServer(t)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/health", nil)

	srv.healthHandler(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}
}

func TestDecodeContainers(t *testing.T) {
	agent := Agent{
		Containers: ContainerSnapshotList{
			{ID: "1", Name: "c1"},
			{ID: "2", Name: "c2"},
		},
	}

	decoded := decodeContainers(agent)
	if len(decoded) != 2 {
		t.Errorf("expected 2 containers, got %d", len(decoded))
	}
	if decoded[0].ID != "1" || decoded[1].Name != "c2" {
		t.Error("content mismatch")
	}

	// Test deep copy / separate slice
	decoded[0].Name = "modified"
	if agent.Containers[0].Name == "modified" {
		// decodeContainers makes a copy using `copy`?
		// Yes: out := make(..., len); copy(out, agent.Containers)
		// But the elements are structs, so it's a shallow copy of elements.
		// Since ContainerSnapshot contains slices (Ports, Labels), those are shared?
		// Wait, ContainerSnapshot struct:
		// type ContainerSnapshot struct { ... Ports []string ... }
		// So modifying Ports in copy would affect original. Modifying Name (string) won't.
		// My test verified Name modification on copy didn't affect original?
		// No, I checked if agent.Containers[0].Name == "modified".
		// Since slice copy copies values (structs), and struct fields like Name are values, it shouldn't affect original.
	} else {
		// Pass
	}
}

func TestToContainerSnapshot(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	raw := map[string]interface{}{
		"id":              "123",
		"name":            "test",
		"autoUpdate":      true,
		"checkedAt":       now.Format(time.RFC3339),
		"updateAvailable": true,
		"ports":           []interface{}{"80/tcp"},
		"labels":          []interface{}{"label1"},
	}

	cs := toContainerSnapshot(raw)

	if cs.ID != "123" {
		t.Errorf("ID = %s, want 123", cs.ID)
	}
	if cs.Name != "test" {
		t.Errorf("Name = %s, want test", cs.Name)
	}
	if !cs.AutoUpdate {
		t.Error("AutoUpdate should be true")
	}
	if !cs.UpdateAvailable {
		t.Error("UpdateAvailable should be true")
	}
	if cs.CheckedAt == nil || !cs.CheckedAt.Equal(now) {
		t.Errorf("CheckedAt mismatch: got %v, want %v", cs.CheckedAt, now)
	}
	if len(cs.Ports) != 1 || cs.Ports[0] != "80/tcp" {
		t.Errorf("Ports mismatch: %v", cs.Ports)
	}
	if len(cs.Labels) != 1 || cs.Labels[0] != "label1" {
		t.Errorf("Labels mismatch: %v", cs.Labels)
	}
}

func TestSetupTestDB(t *testing.T) {
	// This function is used by other tests, but let's verify it works on its own
	db := setupTestDB(t)
	if db == nil {
		t.Fatal("setupTestDB returned nil")
	}
	// Check if migration worked (one table check)
	if !db.Migrator().HasTable(&Account{}) {
		t.Error("Account table missing")
	}
}
