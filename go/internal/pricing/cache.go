package pricing

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const cacheVersion = 1

type CacheStore struct {
	Dir  string
	Path string
}

type CacheData struct {
	Version   int                    `json:"version"`
	UpdatedAt string                 `json:"updated_at,omitempty"`
	Currency  string                 `json:"currency"`
	Entries   map[string]*CacheEntry `json:"entries"`
}

type CacheEntry struct {
	Key            string  `json:"key"`
	SKUName        string  `json:"sku_name"`
	SKUID          string  `json:"sku_id"`
	Description    string  `json:"description"`
	ResourceFamily string  `json:"resource_family,omitempty"`
	ResourceGroup  string  `json:"resource_group,omitempty"`
	UsageType      string  `json:"usage_type,omitempty"`
	Region         string  `json:"region"`
	Unit           string  `json:"unit"`
	Currency       string  `json:"currency"`
	UnitPrice      float64 `json:"unit_price"`
	EffectiveTime  string  `json:"effective_time,omitempty"`
	FetchedAt      string  `json:"fetched_at"`
}

func NewCacheStore(path string) *CacheStore {
	return &CacheStore{
		Dir:  filepath.Dir(path),
		Path: path,
	}
}

func (s *CacheStore) Load() (*CacheData, error) {
	data := &CacheData{
		Version:  cacheVersion,
		Currency: "USD",
		Entries:  map[string]*CacheEntry{},
	}
	raw, err := os.ReadFile(s.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return data, nil
		}
		return nil, fmt.Errorf("read pricing cache: %w", err)
	}
	if err := json.Unmarshal(raw, data); err != nil {
		return nil, fmt.Errorf("parse pricing cache: %w", err)
	}
	if data.Version == 0 {
		data.Version = cacheVersion
	}
	if data.Version > cacheVersion {
		return nil, fmt.Errorf("pricing cache version %d is newer than supported %d", data.Version, cacheVersion)
	}
	if data.Currency == "" {
		data.Currency = "USD"
	}
	if data.Entries == nil {
		data.Entries = map[string]*CacheEntry{}
	}
	return data, nil
}

func (s *CacheStore) Save(data *CacheData) error {
	if data == nil {
		return fmt.Errorf("pricing cache data is nil")
	}
	if err := os.MkdirAll(s.Dir, 0o755); err != nil {
		return fmt.Errorf("create pricing cache dir: %w", err)
	}
	if data.Version == 0 {
		data.Version = cacheVersion
	}
	if data.Currency == "" {
		data.Currency = "USD"
	}
	if data.Entries == nil {
		data.Entries = map[string]*CacheEntry{}
	}
	if data.UpdatedAt == "" {
		data.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("encode pricing cache: %w", err)
	}
	tmp := s.Path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return fmt.Errorf("write pricing cache: %w", err)
	}
	if err := os.Rename(tmp, s.Path); err != nil {
		return fmt.Errorf("replace pricing cache: %w", err)
	}
	return nil
}
