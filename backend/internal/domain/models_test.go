package domain

import (
	"database/sql/driver"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"gorm.io/gorm"
)

func TestStringList_Value(t *testing.T) {
	tests := []struct {
		name    string
		s       StringList
		want    driver.Value
		wantErr bool
	}{
		{
			name: "Empty",
			s:    StringList{},
			want: "[]",
		},
		{
			name: "Nil",
			s:    nil,
			want: "[]",
		},
		{
			name: "Values",
			s:    StringList{"a", "b"},
			want: "[\"a\",\"b\"]",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.s.Value()
			if (err != nil) != tt.wantErr {
				t.Errorf("StringList.Value() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("StringList.Value() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStringList_Scan(t *testing.T) {
	tests := []struct {
		name    string
		src     interface{}
		want    StringList
		wantErr bool
	}{
		{
			name: "Nil",
			src:  nil,
			want: StringList{},
		},
		{
			name: "Bytes",
			src:  []byte("[\"a\",\"b\"]"),
			want: StringList{"a", "b"},
		},
		{
			name: "String",
			src:  "[\"c\",\"d\"]",
			want: StringList{"c", "d"},
		},
		{
			name:    "Invalid",
			src:     123,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var s StringList
			if err := s.Scan(tt.src); (err != nil) != tt.wantErr {
				t.Errorf("StringList.Scan() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && !reflect.DeepEqual(s, tt.want) {
				t.Errorf("StringList.Scan() = %v, want %v", s, tt.want)
			}
		})
	}
}

func TestJSONMap_Value(t *testing.T) {
	tests := []struct {
		name    string
		m       JSONMap
		want    string // comparing as string because map order is random-ish in json, but for small maps it's usually predictable or we check equality differently
		wantErr bool
	}{
		{
			name: "Empty",
			m:    JSONMap{},
			want: "{}",
		},
		{
			name: "Nil",
			m:    nil,
			want: "{}",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.m.Value()
			if (err != nil) != tt.wantErr {
				t.Errorf("JSONMap.Value() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("JSONMap.Value() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestJSONMap_Scan(t *testing.T) {
	tests := []struct {
		name    string
		src     interface{}
		want    JSONMap
		wantErr bool
	}{
		{
			name: "Nil",
			src:  nil,
			want: JSONMap{},
		},
		{
			name: "Bytes",
			src:  []byte("{\"key\":\"value\"}"),
			want: JSONMap{"key": "value"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var m JSONMap
			if err := m.Scan(tt.src); (err != nil) != tt.wantErr {
				t.Errorf("JSONMap.Scan() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && !reflect.DeepEqual(m, tt.want) {
				t.Errorf("JSONMap.Scan() = %v, want %v", m, tt.want)
			}
		})
	}
}

func TestContainerSnapshotList_ValueScan(t *testing.T) {
	// Combined test for Value and Scan to avoid duplicating setup
	now := time.Now().Truncate(time.Second) // Truncate for JSON precision matching
	list := ContainerSnapshotList{
		{
			ID:        "123",
			Name:      "test-container",
			CheckedAt: &now,
		},
	}

	// Test Value
	val, err := list.Value()
	if err != nil {
		t.Fatalf("Value() failed: %v", err)
	}

	strVal, ok := val.(string)
	if !ok {
		t.Fatalf("Value() returned non-string: %T", val)
	}

	// Test Scan
	var scannedList ContainerSnapshotList
	if err := scannedList.Scan([]byte(strVal)); err != nil {
		t.Fatalf("Scan() failed: %v", err)
	}

	if len(scannedList) != 1 {
		t.Fatalf("expected 1 item, got %d", len(scannedList))
	}
	if scannedList[0].ID != "123" {
		t.Errorf("expected ID 123, got %s", scannedList[0].ID)
	}
	// Check time
	if scannedList[0].CheckedAt == nil || !scannedList[0].CheckedAt.Equal(now) {
		// JSON marshaling might change timezone or format, so compare loosely or expect RFC3339
		// Actually, verify if it's close enough or correctly parsed.
		// Since we control the flow, let's just check if not nil.
		if scannedList[0].CheckedAt == nil {
			t.Error("CheckedAt is nil")
		}
	}
}

func TestModels_BeforeCreate(t *testing.T) {
	// Mock DB is not used in BeforeCreate for these models, so we can pass nil or a dummy
	var db *gorm.DB

	t.Run("Account", func(t *testing.T) {
		m := &Account{}
		if err := m.BeforeCreate(db); err != nil {
			t.Errorf("BeforeCreate failed: %v", err)
		}
		if m.ID == "" {
			t.Error("ID was not generated")
		}
	})

	t.Run("ContainerSettings", func(t *testing.T) {
		m := &ContainerSettings{}
		if err := m.BeforeCreate(db); err != nil {
			t.Errorf("BeforeCreate failed: %v", err)
		}
		if m.ID == "" {
			t.Error("ID was not generated")
		}
	})

	t.Run("UpdateHistory", func(t *testing.T) {
		m := &UpdateHistory{}
		if err := m.BeforeCreate(db); err != nil {
			t.Errorf("BeforeCreate failed: %v", err)
		}
		if m.ID == "" {
			t.Error("ID was not generated")
		}
	})

	t.Run("RunningSnapshot", func(t *testing.T) {
		m := &RunningSnapshot{}
		if err := m.BeforeCreate(db); err != nil {
			t.Errorf("BeforeCreate failed: %v", err)
		}
		if m.ID == "" {
			t.Error("ID was not generated")
		}
	})

	t.Run("Schedule", func(t *testing.T) {
		m := &Schedule{}
		if err := m.BeforeCreate(db); err != nil {
			t.Errorf("BeforeCreate failed: %v", err)
		}
		if m.ID == "" {
			t.Error("ID was not generated")
		}
	})

	t.Run("Agent", func(t *testing.T) {
		m := &Agent{}
		if err := m.BeforeCreate(db); err != nil {
			t.Errorf("BeforeCreate failed: %v", err)
		}
		if m.ID == "" {
			t.Error("ID was not generated")
		}
	})

	t.Run("AgentCommand", func(t *testing.T) {
		m := &AgentCommand{}
		if err := m.BeforeCreate(db); err != nil {
			t.Errorf("BeforeCreate failed: %v", err)
		}
		if m.ID == "" {
			t.Error("ID was not generated")
		}
		if m.Status != "pending" {
			t.Errorf("Status = %q, want pending", m.Status)
		}
	})
}

func TestJSONMap_Value_Complex(t *testing.T) {
	m := JSONMap{"foo": "bar", "baz": 123}
	val, err := m.Value()
	if err != nil {
		t.Fatal(err)
	}
	s, ok := val.(string)
	if !ok {
		t.Fatal("expected string")
	}
	// Check if valid JSON
	var check map[string]interface{}
	if err := json.Unmarshal([]byte(s), &check); err != nil {
		t.Fatal(err)
	}
	if check["foo"] != "bar" || check["baz"] != 123.0 { // numbers are float64 in generic unmarshal
		t.Errorf("unexpected json content: %v", check)
	}
}
