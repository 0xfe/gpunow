package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const stateVersion = 1

type Store struct {
	Dir  string
	Path string
}

type Data struct {
	Version   int                 `json:"version"`
	UpdatedAt string              `json:"updated_at"`
	Clusters  map[string]*Cluster `json:"clusters"`
	VMs       map[string]*VM      `json:"vms"`
}

type Cluster struct {
	Name         string `json:"name"`
	Profile      string `json:"profile"`
	NumInstances int    `json:"num_instances"`
	Status       string `json:"status"`
	CreatedAt    string `json:"created_at,omitempty"`
	UpdatedAt    string `json:"updated_at,omitempty"`
	LastAction   string `json:"last_action,omitempty"`
	LastActionAt string `json:"last_action_at,omitempty"`
	DeletedAt    string `json:"deleted_at,omitempty"`
}

type VM struct {
	Name         string `json:"name"`
	Profile      string `json:"profile"`
	Status       string `json:"status"`
	CreatedAt    string `json:"created_at,omitempty"`
	UpdatedAt    string `json:"updated_at,omitempty"`
	LastAction   string `json:"last_action,omitempty"`
	LastActionAt string `json:"last_action_at,omitempty"`
	DeletedAt    string `json:"deleted_at,omitempty"`
}

func New(dir string) *Store {
	return &Store{
		Dir:  dir,
		Path: filepath.Join(dir, "state.json"),
	}
}

func (s *Store) RecordClusterCreate(name, profile string, numInstances int, when time.Time) error {
	data, err := s.load()
	if err != nil {
		return err
	}
	if data.Clusters == nil {
		data.Clusters = map[string]*Cluster{}
	}
	entry := data.Clusters[name]
	if entry == nil {
		entry = &Cluster{Name: name}
		data.Clusters[name] = entry
	}
	ts := when.UTC().Format(time.RFC3339)
	if entry.CreatedAt == "" {
		entry.CreatedAt = ts
	}
	entry.UpdatedAt = ts
	entry.Profile = profile
	entry.NumInstances = numInstances
	entry.Status = "stopped"
	entry.LastAction = "create"
	entry.LastActionAt = ts
	entry.DeletedAt = ""
	data.UpdatedAt = ts
	return s.save(data)
}

func (s *Store) RecordClusterStart(name, profile string, numInstances int, when time.Time) error {
	data, err := s.load()
	if err != nil {
		return err
	}
	if data.Clusters == nil {
		data.Clusters = map[string]*Cluster{}
	}
	entry := data.Clusters[name]
	if entry == nil {
		entry = &Cluster{Name: name}
		data.Clusters[name] = entry
	}
	ts := when.UTC().Format(time.RFC3339)
	if entry.CreatedAt == "" {
		entry.CreatedAt = ts
	}
	entry.UpdatedAt = ts
	entry.Profile = profile
	entry.NumInstances = numInstances
	entry.Status = "running"
	entry.LastAction = "start"
	entry.LastActionAt = ts
	entry.DeletedAt = ""

	data.UpdatedAt = ts
	return s.save(data)
}

func (s *Store) RecordClusterStop(name string, deleted bool, when time.Time) error {
	data, err := s.load()
	if err != nil {
		return err
	}
	if data.Clusters == nil {
		data.Clusters = map[string]*Cluster{}
	}
	entry := data.Clusters[name]
	if entry == nil {
		entry = &Cluster{Name: name}
		data.Clusters[name] = entry
	}
	ts := when.UTC().Format(time.RFC3339)
	entry.UpdatedAt = ts
	if deleted {
		entry.Status = "deleted"
		entry.DeletedAt = ts
		entry.LastAction = "delete"
		entry.LastActionAt = ts
	} else {
		entry.Status = "stopped"
		entry.LastAction = "stop"
		entry.LastActionAt = ts
	}
	data.UpdatedAt = ts
	return s.save(data)
}

func (s *Store) RecordClusterUpdate(name string, when time.Time) error {
	data, err := s.load()
	if err != nil {
		return err
	}
	if data.Clusters == nil {
		data.Clusters = map[string]*Cluster{}
	}
	entry := data.Clusters[name]
	if entry == nil {
		entry = &Cluster{Name: name}
		data.Clusters[name] = entry
	}
	ts := when.UTC().Format(time.RFC3339)
	entry.UpdatedAt = ts
	entry.LastAction = "update"
	entry.LastActionAt = ts
	data.UpdatedAt = ts
	return s.save(data)
}

func (s *Store) DeleteCluster(name string) error {
	data, err := s.load()
	if err != nil {
		return err
	}
	if data.Clusters != nil {
		delete(data.Clusters, name)
	}
	data.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	return s.save(data)
}

func (s *Store) RecordVMStart(name, profile string, when time.Time) error {
	data, err := s.load()
	if err != nil {
		return err
	}
	if data.VMs == nil {
		data.VMs = map[string]*VM{}
	}
	entry := data.VMs[name]
	if entry == nil {
		entry = &VM{Name: name}
		data.VMs[name] = entry
	}
	ts := when.UTC().Format(time.RFC3339)
	if entry.CreatedAt == "" {
		entry.CreatedAt = ts
	}
	entry.UpdatedAt = ts
	entry.Profile = profile
	entry.Status = "running"
	entry.LastAction = "start"
	entry.LastActionAt = ts
	entry.DeletedAt = ""

	data.UpdatedAt = ts
	return s.save(data)
}

func (s *Store) RecordVMStop(name string, deleted bool, when time.Time) error {
	data, err := s.load()
	if err != nil {
		return err
	}
	if data.VMs == nil {
		data.VMs = map[string]*VM{}
	}
	entry := data.VMs[name]
	if entry == nil {
		entry = &VM{Name: name}
		data.VMs[name] = entry
	}
	ts := when.UTC().Format(time.RFC3339)
	entry.UpdatedAt = ts
	if deleted {
		entry.Status = "deleted"
		entry.DeletedAt = ts
		entry.LastAction = "delete"
		entry.LastActionAt = ts
	} else {
		entry.Status = "stopped"
		entry.LastAction = "stop"
		entry.LastActionAt = ts
	}
	data.UpdatedAt = ts
	return s.save(data)
}

func (s *Store) RecordVMUpdate(name string, when time.Time) error {
	data, err := s.load()
	if err != nil {
		return err
	}
	if data.VMs == nil {
		data.VMs = map[string]*VM{}
	}
	entry := data.VMs[name]
	if entry == nil {
		entry = &VM{Name: name}
		data.VMs[name] = entry
	}
	ts := when.UTC().Format(time.RFC3339)
	entry.UpdatedAt = ts
	entry.LastAction = "update"
	entry.LastActionAt = ts
	data.UpdatedAt = ts
	return s.save(data)
}

func (s *Store) Load() (*Data, error) {
	return s.load()
}

func (s *Store) Save(data *Data) error {
	return s.save(data)
}

func (s *Store) load() (*Data, error) {
	data := &Data{
		Version:  stateVersion,
		Clusters: map[string]*Cluster{},
		VMs:      map[string]*VM{},
	}
	raw, err := os.ReadFile(s.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return data, nil
		}
		return nil, fmt.Errorf("read state: %w", err)
	}
	if err := json.Unmarshal(raw, data); err != nil {
		return nil, fmt.Errorf("parse state: %w", err)
	}
	if data.Version == 0 {
		data.Version = stateVersion
	}
	if data.Version > stateVersion {
		return nil, fmt.Errorf("state version %d is newer than supported %d", data.Version, stateVersion)
	}
	if data.Clusters == nil {
		data.Clusters = map[string]*Cluster{}
	}
	if data.VMs == nil {
		data.VMs = map[string]*VM{}
	}
	return data, nil
}

func (s *Store) save(data *Data) error {
	if err := os.MkdirAll(s.Dir, 0o755); err != nil {
		return fmt.Errorf("create state dir: %w", err)
	}
	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("encode state: %w", err)
	}
	tmp := s.Path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return fmt.Errorf("write state: %w", err)
	}
	if err := os.Rename(tmp, s.Path); err != nil {
		return fmt.Errorf("replace state: %w", err)
	}
	return nil
}
