package cli

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/compute/apiv1/computepb"
	"github.com/urfave/cli/v2"
	"google.golang.org/api/iterator"
	"google.golang.org/protobuf/proto"

	"gpunow/internal/cluster"
	"gpunow/internal/gcp"
	"gpunow/internal/labels"
	"gpunow/internal/lifecycle"
	"gpunow/internal/ssh"
	appstate "gpunow/internal/state"
)

func statusCommand() *cli.Command {
	return &cli.Command{
		Name:      "status",
		Usage:     "Show status of gpunow clusters",
		ArgsUsage: "[cluster]",
		Action:    statusShow,
		Subcommands: []*cli.Command{
			{
				Name:   "sync",
				Usage:  "Sync status from GCP into local state",
				Action: statusSync,
			},
		},
	}
}

func statusShow(c *cli.Context) error {
	state, err := GetState(c)
	if err != nil {
		return err
	}
	selection, selErr := ssh.ResolvePublicKeySelection(state.Config.SSH.IdentityFile)
	if c.Args().Len() > 0 {
		clusterName := strings.TrimSpace(c.Args().First())
		if clusterName == "" {
			return nil
		}
		if selErr != nil {
			announce(state)
			state.UI.Warnf("ssh key: %v", selErr)
		} else {
			announceWithKey(state, selection, true)
		}
		compute, err := state.ComputeClient(c.Context)
		if err != nil {
			return err
		}
		service := cluster.NewService(compute, state.Config, state.UI, state.Logger)
		return service.Show(c.Context, clusterName)
	}
	if selErr != nil {
		announceStatus(state)
		state.UI.Warnf("ssh key: %v", selErr)
	} else {
		announceStatusWithKey(state, selection, true)
	}

	data, err := state.State.Load()
	if err != nil {
		return err
	}

	compute, err := state.ComputeClient(c.Context)
	if err != nil {
		state.UI.Warnf("Live instance lookup unavailable: %v", err)
		renderStatus(state, data, nil)
		return nil
	}
	instancesByCluster, err := clusterInstancesForStatus(c.Context, state, compute, data)
	if err != nil {
		state.UI.Warnf("Live instance lookup unavailable: %v", err)
		renderStatus(state, data, nil)
		return nil
	}
	renderStatus(state, data, instancesByCluster)
	return nil
}

func statusSync(c *cli.Context) error {
	state, err := GetState(c)
	if err != nil {
		return err
	}
	announceStatus(state)

	compute, err := state.ComputeClient(c.Context)
	if err != nil {
		return err
	}

	data, err := state.State.Load()
	if err != nil {
		return err
	}

	now := time.Now().UTC().Format(time.RFC3339)
	foundClusters := map[string]*appstate.Cluster{}
	clusterAgg := map[string]*clusterStatusAgg{}
	clusterInstances := map[string]map[string]*appstate.ClusterInstance{}

	filter := labels.Filter()
	it := compute.ListInstances(c.Context, &computepb.ListInstancesRequest{
		Project: state.Config.Project.ID,
		Zone:    state.Config.Project.Zone,
		Filter:  proto.String(filter),
	})

	for {
		inst, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return err
		}
		instLabels := inst.GetLabels()
		if instLabels == nil || instLabels[labels.ManagedKey] != labels.ManagedValue {
			continue
		}

		clusterName := strings.TrimSpace(instLabels["cluster"])
		if clusterName == "" {
			continue
		}
		agg := clusterAgg[clusterName]
		if agg == nil {
			agg = &clusterStatusAgg{}
			clusterAgg[clusterName] = agg
		}
		agg.total++
		agg.observe(inst.GetStatus())
		if clusterInstances[clusterName] == nil {
			clusterInstances[clusterName] = map[string]*appstate.ClusterInstance{}
		}
		index := parseInstanceIndex(clusterName, inst.GetName())
		if rawIndex, ok := instLabels["cluster_index"]; ok {
			if parsed, err := strconv.Atoi(strings.TrimSpace(rawIndex)); err == nil && parsed >= 0 {
				index = parsed
			}
		}
		clusterInstances[clusterName][inst.GetName()] = &appstate.ClusterInstance{
			Name:       inst.GetName(),
			Index:      index,
			State:      lifecycle.FromComputeStatus(inst.GetStatus()),
			ExternalIP: externalIPFromInstance(inst),
			InternalIP: internalIPFromInstance(inst),
			UpdatedAt:  now,
		}
	}

	for name, agg := range clusterAgg {
		entry := foundClusters[name]
		if entry == nil {
			entry = existingCluster(data, name)
			if entry.Name == "" {
				entry.Name = name
			}
			if entry.Profile == "" {
				entry.Profile = state.Profile
			}
			if entry.CreatedAt == "" {
				entry.CreatedAt = now
			}
		}
		entry.NumInstances = agg.total
		entry.Status = agg.status()
		entry.Instances = clusterInstances[name]
		entry.UpdatedAt = now
		foundClusters[name] = entry
	}

	for name, entry := range data.Clusters {
		if entry == nil {
			continue
		}
		if _, ok := foundClusters[name]; ok {
			continue
		}
		instances, err := listInstancesByFilter(c.Context, compute, state.Config.Project.ID, state.Config.Project.Zone, fmt.Sprintf("labels.cluster = %q", name))
		if err != nil {
			return err
		}
		if len(instances) > 0 {
			agg := &clusterStatusAgg{}
			instanceMap := map[string]*appstate.ClusterInstance{}
			for _, inst := range instances {
				agg.total++
				agg.observe(inst.GetStatus())
				instanceMap[inst.GetName()] = &appstate.ClusterInstance{
					Name:       inst.GetName(),
					Index:      parseInstanceIndex(name, inst.GetName()),
					State:      lifecycle.FromComputeStatus(inst.GetStatus()),
					ExternalIP: externalIPFromInstance(inst),
					InternalIP: internalIPFromInstance(inst),
					UpdatedAt:  now,
				}
			}
			refreshed := existingCluster(data, name)
			if refreshed.Name == "" {
				refreshed.Name = name
			}
			if refreshed.Profile == "" {
				refreshed.Profile = state.Profile
			}
			if refreshed.CreatedAt == "" {
				refreshed.CreatedAt = now
			}
			refreshed.NumInstances = agg.total
			refreshed.Status = agg.status()
			refreshed.Instances = instanceMap
			refreshed.UpdatedAt = now
			foundClusters[name] = refreshed
			continue
		}
		if entry.LastAction == "create" && entry.Status != "deleted" {
			if entry.Instances == nil {
				entry.Instances = map[string]*appstate.ClusterInstance{}
				for i := 0; i < entry.NumInstances; i++ {
					instanceName := fmt.Sprintf("%s-%d", name, i)
					entry.Instances[instanceName] = &appstate.ClusterInstance{
						Name:      instanceName,
						Index:     i,
						State:     lifecycle.InstanceStateTerminated,
						UpdatedAt: now,
					}
				}
			}
			entry.Status = lifecycle.InstanceStateTerminated
			entry.UpdatedAt = now
			foundClusters[name] = entry
			continue
		}
		entry = markDeletedCluster(entry, now)
		foundClusters[name] = entry
	}
	data.Clusters = foundClusters
	data.UpdatedAt = now

	if err := state.State.Save(data); err != nil {
		return err
	}

	state.UI.Successf("Synced %d clusters", len(foundClusters))
	instancesByCluster, err := clusterInstancesForStatus(c.Context, state, compute, data)
	if err != nil {
		state.UI.Warnf("Live instance lookup unavailable: %v", err)
		renderStatus(state, data, nil)
		return nil
	}
	renderStatus(state, data, instancesByCluster)
	return nil
}

func sortedKeys[T any](m map[string]T) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func defaultProfile(profile string) string {
	if profile == "" {
		return "default"
	}
	return profile
}

func renderStatus(state *State, data *appstate.Data, instancesByCluster map[string][]*computepb.Instance) {
	totalInstances := 0
	activeClusters := 0
	for _, entry := range data.Clusters {
		if entry == nil {
			continue
		}
		if entry.Status == "deleted" {
			continue
		}
		activeClusters++
		if len(entry.Instances) > 0 {
			totalInstances += len(entry.Instances)
		} else {
			totalInstances += entry.NumInstances
		}
	}

	state.UI.Heading("Status")
	state.UI.Infof("File: %s", state.State.Path)
	if data.UpdatedAt != "" {
		state.UI.Infof("Updated: %s", data.UpdatedAt)
	} else {
		state.UI.Infof("Updated: (none)")
	}
	state.UI.Infof("Instances: %d", totalInstances)
	state.UI.Infof("Clusters: %d", activeClusters)

	if activeClusters > 0 {
		state.UI.Heading("Clusters")
		clusterKeys := sortedKeys(data.Clusters)
		for _, name := range clusterKeys {
			entry := data.Clusters[name]
			if entry == nil {
				continue
			}
			if entry.Status == "deleted" {
				continue
			}
			profile := defaultProfile(entry.Profile)
			line := fmt.Sprintf("%s (%s) %s", entry.Name, profile, entry.Status)
			state.UI.Infof("%s", line)
			instanceCount := entry.NumInstances
			if len(entry.Instances) > 0 {
				instanceCount = len(entry.Instances)
			}
			state.UI.InfofIndent(1, "Instances: %d", instanceCount)
			if overrideSummary := clusterConfigSummary(entry.Config); overrideSummary != "" {
				state.UI.InfofIndent(1, "Overrides: %s", overrideSummary)
			}
			for _, instance := range renderedInstances(entry, instancesByCluster[entry.Name]) {
				line := fmt.Sprintf("%s (%s)", instance.Name, instance.State)
				if instance.ExternalIP != "" {
					line = fmt.Sprintf("%s %s", line, instance.ExternalIP)
				}
				if instance.InternalIP != "" {
					line = fmt.Sprintf("%s [%s]", line, instance.InternalIP)
				}
				state.UI.InfofIndent(1, "%s", line)
			}
			if entry.LastAction != "" {
				state.UI.InfofIndent(1, "Last action: %s (%s)", entry.LastAction, entry.LastActionAt)
			}
			if entry.DeletedAt != "" {
				state.UI.InfofIndent(1, "Deleted: %s", entry.DeletedAt)
			}
		}
	}
}

type statusInstanceLine struct {
	Name       string
	State      string
	ExternalIP string
	InternalIP string
	Index      int
}

func renderedInstances(entry *appstate.Cluster, live []*computepb.Instance) []statusInstanceLine {
	byName := map[string]statusInstanceLine{}
	for name, instance := range entry.Instances {
		if instance == nil {
			continue
		}
		line := statusInstanceLine{
			Name:       name,
			State:      lifecycle.NormalizeInstanceState(instance.State),
			ExternalIP: instance.ExternalIP,
			InternalIP: instance.InternalIP,
			Index:      instance.Index,
		}
		if line.State == "" {
			line.State = lifecycle.InstanceStateTerminated
		}
		byName[name] = line
	}
	for _, inst := range live {
		if inst == nil {
			continue
		}
		name := inst.GetName()
		line := byName[name]
		if line.Name == "" {
			line.Name = name
			line.State = lifecycle.FromComputeStatus(inst.GetStatus())
			line.Index = parseInstanceIndex(entry.Name, name)
		}
		if line.ExternalIP == "" {
			line.ExternalIP = externalIPFromInstance(inst)
		}
		if line.InternalIP == "" {
			line.InternalIP = internalIPFromInstance(inst)
		}
		byName[name] = line
	}

	out := make([]statusInstanceLine, 0, len(byName))
	for _, line := range byName {
		if line.Name == "" {
			continue
		}
		if line.State == "" {
			line.State = lifecycle.InstanceStateTerminated
		}
		if line.Index < 0 {
			line.Index = parseInstanceIndex(entry.Name, line.Name)
		}
		out = append(out, line)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Index != out[j].Index {
			if out[i].Index < 0 {
				return false
			}
			if out[j].Index < 0 {
				return true
			}
			return out[i].Index < out[j].Index
		}
		return out[i].Name < out[j].Name
	})
	return out
}

type clusterStatusAgg struct {
	total          int
	anyReady       bool
	anyTerminated  bool
	anyStarting    bool
	anyTerminating bool
}

func (c *clusterStatusAgg) observe(status string) {
	switch lifecycle.FromComputeStatus(status) {
	case lifecycle.InstanceStateReady:
		c.anyReady = true
	case lifecycle.InstanceStateTerminated:
		c.anyTerminated = true
	case lifecycle.InstanceStateTerminating:
		c.anyTerminating = true
	default:
		c.anyStarting = true
	}
}

func (c *clusterStatusAgg) status() string {
	if c.total == 0 {
		return lifecycle.InstanceStateTerminated
	}
	if c.anyTerminating {
		return lifecycle.InstanceStateTerminating
	}
	if c.anyStarting {
		return lifecycle.InstanceStateStarting
	}
	if c.anyReady && !c.anyTerminated {
		return lifecycle.InstanceStateReady
	}
	if c.anyReady {
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

func existingCluster(data *appstate.Data, name string) *appstate.Cluster {
	if data == nil || data.Clusters == nil {
		return &appstate.Cluster{}
	}
	if entry := data.Clusters[name]; entry != nil {
		return entry
	}
	return &appstate.Cluster{}
}

func markDeletedCluster(entry *appstate.Cluster, now string) *appstate.Cluster {
	if entry == nil {
		entry = &appstate.Cluster{}
	}
	if entry.Status != "deleted" {
		entry.Status = "deleted"
		entry.DeletedAt = now
	}
	entry.UpdatedAt = now
	return entry
}

func listInstancesByFilter(ctx context.Context, compute gcp.Compute, project, zone, filter string) ([]*computepb.Instance, error) {
	it := compute.ListInstances(ctx, &computepb.ListInstancesRequest{
		Project: project,
		Zone:    zone,
		Filter:  proto.String(filter),
	})
	var instances []*computepb.Instance
	for {
		inst, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		instances = append(instances, inst)
	}
	return instances, nil
}

func clusterInstancesForStatus(ctx context.Context, state *State, compute gcp.Compute, data *appstate.Data) (map[string][]*computepb.Instance, error) {
	out := map[string][]*computepb.Instance{}
	if data == nil {
		return out, nil
	}
	for _, name := range sortedKeys(data.Clusters) {
		entry := data.Clusters[name]
		if entry == nil || entry.Status == "deleted" {
			continue
		}
		instances, err := listInstancesByFilter(ctx, compute, state.Config.Project.ID, state.Config.Project.Zone, fmt.Sprintf("labels.cluster = %q", name))
		if err != nil {
			return nil, err
		}
		if len(instances) > 0 {
			out[name] = instances
		}
	}
	return out, nil
}

func externalIPFromInstance(inst *computepb.Instance) string {
	if inst == nil || len(inst.GetNetworkInterfaces()) == 0 {
		return ""
	}
	iface := inst.GetNetworkInterfaces()[0]
	if len(iface.GetAccessConfigs()) == 0 {
		return ""
	}
	return iface.GetAccessConfigs()[0].GetNatIP()
}

func internalIPFromInstance(inst *computepb.Instance) string {
	if inst == nil || len(inst.GetNetworkInterfaces()) == 0 {
		return ""
	}
	return inst.GetNetworkInterfaces()[0].GetNetworkIP()
}
