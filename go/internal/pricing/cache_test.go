package pricing

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCacheStoreSaveLoadRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	store := NewCacheStore(filepath.Join(tmp, "pricing-cache.json"))

	data := &CacheData{
		Version:  1,
		Currency: "USD",
		Entries: map[string]*CacheEntry{
			"compute.core.g2-standard-16.spot.us-east1": {
				Key:       "compute.core.g2-standard-16.spot.us-east1",
				SKUID:     "sku-core",
				Unit:      "h",
				Currency:  "USD",
				UnitPrice: 0.05,
			},
		},
	}
	if err := store.Save(data); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.Currency != "USD" {
		t.Fatalf("currency mismatch: got=%s", loaded.Currency)
	}
	if loaded.Entries["compute.core.g2-standard-16.spot.us-east1"] == nil {
		t.Fatalf("expected cached entry")
	}
	if loaded.UpdatedAt == "" {
		t.Fatalf("expected updated_at")
	}
}

func TestCacheStoreLoadMissingReturnsDefaults(t *testing.T) {
	tmp := t.TempDir()
	store := NewCacheStore(filepath.Join(tmp, "pricing-cache.json"))

	data, err := store.Load()
	if err != nil {
		t.Fatalf("load missing: %v", err)
	}
	if data.Version != cacheVersion {
		t.Fatalf("version mismatch: got=%d want=%d", data.Version, cacheVersion)
	}
	if len(data.Entries) != 0 {
		t.Fatalf("expected empty entries")
	}
}

func TestCacheStoreRejectsNewerVersion(t *testing.T) {
	tmp := t.TempDir()
	store := NewCacheStore(filepath.Join(tmp, "pricing-cache.json"))

	raw := []byte(`{"version":999,"currency":"USD","entries":{}}`)
	if err := os.WriteFile(store.Path, raw, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	if _, err := store.Load(); err == nil {
		t.Fatalf("expected version error")
	}
}
