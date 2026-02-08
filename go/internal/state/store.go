package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gpunow/internal/lifecycle"
)

const stateVersion = 3

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
	Name         string                      `json:"name"`
	Profile      string                      `json:"profile"`
	NumInstances int                         `json:"num_instances"`
	Config       ClusterConfig               `json:"config,omitempty"`
	Instances    map[string]*ClusterInstance `json:"instances,omitempty"`
	Status       string                      `json:"status"`
	CreatedAt    string                      `json:"created_at,omitempty"`
	UpdatedAt    string                      `json:"updated_at,omitempty"`
	LastAction   string                      `json:"last_action,omitempty"`
	LastActionAt string                      `json:"last_action_at,omitempty"`
	DeletedAt    string                      `json:"deleted_at,omitempty"`
}

type ClusterInstance struct {
	Name       string `json:"name"`
	Index      int    `json:"index"`
	State      string `json:"state"`
	ExternalIP string `json:"external_ip,omitempty"`
	InternalIP string `json:"internal_ip,omitempty"`
	CreatedAt  string `json:"created_at,omitempty"`
	UpdatedAt  string `json:"updated_at,omitempty"`
}

type ClusterConfig struct {
	GCPMachineType       string `json:"gcp_machine_type,omitempty"`
	GCPMaxRunHours       int    `json:"gcp_max_run_hours,omitempty"`
	GCPTerminationAction string `json:"gcp_termination_action,omitempty"`
	GCPDiskSizeGB        int    `json:"gcp_disk_size_gb,omitempty"`
	KeepDisks            bool   `json:"keep_disks,omitempty"`
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

func (s *Store) RecordClusterCreate(name, profile string, numInstances int, clusterConfig ClusterConfig, when time.Time) error {
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
	entry.Config = clusterConfig
	entry.Instances = ensureClusterInstances(name, entry.Instances, numInstances, ts)
	entry.Status = deriveClusterState(entry.Instances, entry.NumInstances)
	entry.LastAction = "create"
	entry.LastActionAt = ts
	entry.DeletedAt = ""
	data.UpdatedAt = ts
	return s.save(data)
}

func (s *Store) RecordClusterStart(name, profile string, numInstances int, clusterConfig ClusterConfig, when time.Time) error {
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
	entry.Config = clusterConfig
	entry.Instances = ensureClusterInstances(name, entry.Instances, numInstances, ts)
	entry.Status = deriveClusterState(entry.Instances, entry.NumInstances)
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
		for _, instance := range entry.Instances {
			if instance == nil {
				continue
			}
			instance.State = lifecycle.InstanceStateTerminated
			instance.ExternalIP = ""
			instance.InternalIP = ""
			instance.UpdatedAt = ts
		}
		entry.Status = deriveClusterState(entry.Instances, entry.NumInstances)
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

func (s *Store) RecordClusterInstanceState(clusterName, instanceName, instanceState, externalIP, internalIP string, when time.Time) error {
	data, err := s.load()
	if err != nil {
		return err
	}
	if data.Clusters == nil {
		data.Clusters = map[string]*Cluster{}
	}
	entry := data.Clusters[clusterName]
	if entry == nil {
		entry = &Cluster{Name: clusterName}
		data.Clusters[clusterName] = entry
	}
	ts := when.UTC().Format(time.RFC3339)
	if entry.CreatedAt == "" {
		entry.CreatedAt = ts
	}
	entry.UpdatedAt = ts
	entry.DeletedAt = ""
	entry.Instances = ensureClusterInstances(clusterName, entry.Instances, entry.NumInstances, ts)

	instanceEntry := entry.Instances[instanceName]
	if instanceEntry == nil {
		instanceEntry = &ClusterInstance{
			Name:      instanceName,
			Index:     parseInstanceIndex(clusterName, instanceName),
			CreatedAt: ts,
		}
		entry.Instances[instanceName] = instanceEntry
	}
	instanceEntry.State = lifecycle.NormalizeInstanceState(instanceState)
	instanceEntry.UpdatedAt = ts
	if externalIP != "" || instanceEntry.State == lifecycle.InstanceStateTerminated {
		instanceEntry.ExternalIP = externalIP
	}
	if internalIP != "" || instanceEntry.State == lifecycle.InstanceStateTerminated {
		instanceEntry.InternalIP = internalIP
	}
	if instanceEntry.CreatedAt == "" {
		instanceEntry.CreatedAt = ts
	}

	if entry.NumInstances < len(entry.Instances) {
		entry.NumInstances = len(entry.Instances)
	}
	entry.Status = deriveClusterState(entry.Instances, entry.NumInstances)
	data.UpdatedAt = ts
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
	for name, cluster := range data.Clusters {
		if cluster == nil {
			cluster = &Cluster{Name: name}
			data.Clusters[name] = cluster
		}
		if cluster.Name == "" {
			cluster.Name = name
		}
		cluster.Instances = ensureClusterInstances(cluster.Name, cluster.Instances, cluster.NumInstances, cluster.CreatedAt)
		for instanceName, instance := range cluster.Instances {
			if instance == nil {
				instance = &ClusterInstance{Name: instanceName}
				cluster.Instances[instanceName] = instance
			}
			if instance.Name == "" {
				instance.Name = instanceName
			}
			instance.Index = parseInstanceIndex(cluster.Name, instance.Name)
			if instance.State == "" {
				instance.State = lifecycle.InstanceStateTerminated
			} else {
				instance.State = lifecycle.NormalizeInstanceState(instance.State)
			}
		}
		if cluster.Status == "" || cluster.Status == "running" || cluster.Status == "stopped" {
			cluster.Status = deriveClusterState(cluster.Instances, cluster.NumInstances)
		}
	}
	if data.VMs == nil {
		data.VMs = map[string]*VM{}
	}
	return data, nil
}

func (s *Store) save(data *Data) error {
	if data == nil {
		data = &Data{}
	}
	if data.Version < stateVersion {
		data.Version = stateVersion
	}
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

func ensureClusterInstances(clusterName string, existing map[string]*ClusterInstance, numInstances int, ts string) map[string]*ClusterInstance {
	if existing == nil {
		existing = map[string]*ClusterInstance{}
	}
	if numInstances <= 0 {
		return existing
	}
	for i := 0; i < numInstances; i++ {
		name := fmt.Sprintf("%s-%d", clusterName, i)
		entry := existing[name]
		if entry == nil {
			entry = &ClusterInstance{
				Name:      name,
				Index:     i,
				State:     lifecycle.InstanceStateTerminated,
				CreatedAt: ts,
				UpdatedAt: ts,
			}
			existing[name] = entry
			continue
		}
		if entry.Name == "" {
			entry.Name = name
		}
		entry.Index = i
		if entry.State == "" {
			entry.State = lifecycle.InstanceStateTerminated
		}
		if entry.CreatedAt == "" {
			entry.CreatedAt = ts
		}
		if entry.UpdatedAt == "" {
			entry.UpdatedAt = ts
		}
	}
	return existing
}

func deriveClusterState(instances map[string]*ClusterInstance, configuredCount int) string {
	if configuredCount <= 0 && len(instances) == 0 {
		return lifecycle.InstanceStateTerminated
	}
	terminatedCount := 0
	readyCount := 0
	hasStarting := false
	hasProvisioning := false
	hasTerminating := false
	for _, instance := range instances {
		if instance == nil {
			continue
		}
		switch lifecycle.NormalizeInstanceState(instance.State) {
		case lifecycle.InstanceStateStarting:
			hasStarting = true
		case lifecycle.InstanceStateProvisioning:
			hasProvisioning = true
		case lifecycle.InstanceStateTerminating:
			hasTerminating = true
		case lifecycle.InstanceStateReady:
			readyCount++
		case lifecycle.InstanceStateTerminated:
			terminatedCount++
		}
	}
	if hasTerminating {
		return lifecycle.InstanceStateTerminating
	}
	if hasStarting {
		return lifecycle.InstanceStateStarting
	}
	if hasProvisioning {
		return lifecycle.InstanceStateProvisioning
	}
	if readyCount > 0 && terminatedCount == 0 {
		return lifecycle.InstanceStateReady
	}
	if readyCount > 0 {
		return lifecycle.InstanceStateStarting
	}
	return lifecycle.InstanceStateTerminated
}

func parseInstanceIndex(clusterName, instanceName string) int {
	prefix := clusterName + "-"
	if !strings.HasPrefix(instanceName, prefix) {
		return -1
	}
	raw := strings.TrimPrefix(instanceName, prefix)
	idx, err := strconv.Atoi(raw)
	if err != nil || idx < 0 {
		return -1
	}
	return idx
}
