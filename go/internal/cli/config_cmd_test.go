package cli

import (
	"strings"
	"testing"
)

func TestSetTOMLStringKey(t *testing.T) {
	input := "[project]\nid = \"old\"\nzone = \"us-central1-a\"\n\n[instance]\nmachine_type = \"g2-standard-16\"\n"
	updated, err := setTOMLStringKey(input, "project", "id", "new-project")
	if err != nil {
		t.Fatalf("set key: %v", err)
	}
	if !strings.Contains(updated, "id = \"new-project\"") {
		t.Fatalf("expected updated project id, got:\n%s", updated)
	}

	updated, err = setTOMLStringKey(updated, "project", "region", "us-central1")
	if err != nil {
		t.Fatalf("insert key: %v", err)
	}
	if !strings.Contains(updated, "region = \"us-central1\"") {
		t.Fatalf("expected inserted region key, got:\n%s", updated)
	}
}

func TestSetTOMLIntKey(t *testing.T) {
	input := "[instance]\nmax_run_hours = 12\n\n[disk]\nsize_gb = 200\n"
	updated, err := setTOMLIntKey(input, "instance", "max_run_hours", 24)
	if err != nil {
		t.Fatalf("set int key: %v", err)
	}
	if !strings.Contains(updated, "max_run_hours = 24") {
		t.Fatalf("expected updated max_run_hours, got:\n%s", updated)
	}

	updated, err = setTOMLIntKey(updated, "disk", "size_gb", 500)
	if err != nil {
		t.Fatalf("set disk size: %v", err)
	}
	if !strings.Contains(updated, "size_gb = 500") {
		t.Fatalf("expected updated size_gb, got:\n%s", updated)
	}
}
