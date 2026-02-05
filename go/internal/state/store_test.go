package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStoreRecordClusterLifecycle(t *testing.T) {
	tmp := t.TempDir()
	store := New(tmp)

	when := time.Date(2026, 2, 5, 20, 0, 0, 0, time.UTC)
	if err := store.RecordClusterStart("alpha", "default", 3, when); err != nil {
		t.Fatalf("record start: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(tmp, "state.json"))
	if err != nil {
		t.Fatalf("read state: %v", err)
	}
	var data Data
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatalf("parse state: %v", err)
	}
	entry := data.Clusters["alpha"]
	if entry == nil {
		t.Fatalf("expected cluster entry")
	}
	if entry.Profile != "default" || entry.NumInstances != 3 || entry.Status != "running" {
		t.Fatalf("unexpected entry: %+v", entry)
	}

	if err := store.RecordClusterStop("alpha", true, when.Add(1*time.Hour)); err != nil {
		t.Fatalf("record stop: %v", err)
	}
	raw, err = os.ReadFile(filepath.Join(tmp, "state.json"))
	if err != nil {
		t.Fatalf("read state: %v", err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatalf("parse state: %v", err)
	}
	entry = data.Clusters["alpha"]
	if entry == nil || entry.Status != "deleted" || entry.DeletedAt == "" {
		t.Fatalf("unexpected deleted entry: %+v", entry)
	}
}

func TestStoreLoadMissingFile(t *testing.T) {
	tmp := t.TempDir()
	store := New(tmp)
	data, err := store.Load()
	if err != nil {
		t.Fatalf("load missing: %v", err)
	}
	if data == nil || data.Version == 0 {
		t.Fatalf("expected default data")
	}
}
