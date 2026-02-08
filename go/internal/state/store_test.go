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
	if err := store.RecordClusterCreate("alpha", "default", 3, ClusterConfig{}, when.Add(-time.Hour)); err != nil {
		t.Fatalf("record create: %v", err)
	}
	cfg := ClusterConfig{
		GCPMachineType:       "g2-standard-16",
		GCPMaxRunHours:       12,
		GCPTerminationAction: "DELETE",
		GCPDiskSizeGB:        200,
		KeepDisks:            true,
	}
	if err := store.RecordClusterStart("alpha", "default", 3, cfg, when); err != nil {
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
	if entry.LastAction != "start" {
		t.Fatalf("expected last action start, got: %+v", entry)
	}
	if !entry.Config.KeepDisks {
		t.Fatalf("expected keep_disks in config, got: %+v", entry.Config)
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

func TestStoreRecordClusterCreate(t *testing.T) {
	tmp := t.TempDir()
	store := New(tmp)

	when := time.Date(2026, 2, 6, 10, 0, 0, 0, time.UTC)
	if err := store.RecordClusterCreate("beta", "default", 2, ClusterConfig{}, when); err != nil {
		t.Fatalf("record create: %v", err)
	}
	data, err := store.Load()
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	entry := data.Clusters["beta"]
	if entry == nil {
		t.Fatalf("expected cluster entry")
	}
	if entry.Status != "stopped" || entry.LastAction != "create" || entry.NumInstances != 2 {
		t.Fatalf("unexpected entry: %+v", entry)
	}
}

func TestStoreRecordVMLifecycle(t *testing.T) {
	tmp := t.TempDir()
	store := New(tmp)

	when := time.Date(2026, 2, 5, 20, 0, 0, 0, time.UTC)
	if err := store.RecordVMStart("gpu0", "default", when); err != nil {
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
	entry := data.VMs["gpu0"]
	if entry == nil {
		t.Fatalf("expected vm entry")
	}
	if entry.Profile != "default" || entry.Status != "running" {
		t.Fatalf("unexpected entry: %+v", entry)
	}

	if err := store.RecordVMStop("gpu0", true, when.Add(1*time.Hour)); err != nil {
		t.Fatalf("record stop: %v", err)
	}
	raw, err = os.ReadFile(filepath.Join(tmp, "state.json"))
	if err != nil {
		t.Fatalf("read state: %v", err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatalf("parse state: %v", err)
	}
	entry = data.VMs["gpu0"]
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

func TestStoreLoadRejectsNewerVersion(t *testing.T) {
	tmp := t.TempDir()
	store := New(tmp)
	raw := []byte(`{"version":999,"updated_at":"","clusters":{},"vms":{}}`)
	if err := os.WriteFile(store.Path, raw, 0o644); err != nil {
		t.Fatalf("write state: %v", err)
	}
	if _, err := store.Load(); err == nil {
		t.Fatalf("expected version error")
	}
}
