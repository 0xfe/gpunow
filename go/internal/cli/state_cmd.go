package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/urfave/cli/v2"

	appstate "gpunow/internal/state"
)

func stateCommand() *cli.Command {
	return &cli.Command{
		Name:  "state",
		Usage: "Show local state",
		Action: func(c *cli.Context) error {
			return stateShow(c)
		},
		Subcommands: []*cli.Command{
			{
				Name:   "show",
				Usage:  "Show state summary",
				Action: stateShow,
			},
			{
				Name:   "raw",
				Usage:  "Print raw state JSON",
				Action: stateRaw,
			},
		},
	}
}

func stateShow(c *cli.Context) error {
	state, err := GetState(c)
	if err != nil {
		return err
	}
	announce(state)

	data, err := state.State.Load()
	if err != nil {
		return err
	}

	state.UI.Heading("State")
	state.UI.Infof("File: %s", state.State.Path)
	if data.UpdatedAt != "" {
		state.UI.Infof("Updated: %s", data.UpdatedAt)
	} else {
		state.UI.Infof("Updated: (none)")
	}
	state.UI.Infof("Clusters: %d", len(data.Clusters))

	if len(data.Clusters) == 0 {
		state.UI.Infof("No clusters recorded")
		return nil
	}

	keys := make([]string, 0, len(data.Clusters))
	for name := range data.Clusters {
		keys = append(keys, name)
	}
	sort.Strings(keys)

	for _, name := range keys {
		entry := data.Clusters[name]
		if entry == nil {
			continue
		}
		profile := entry.Profile
		if profile == "" {
			profile = "default"
		}
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
		for _, instance := range renderedInstances(entry, nil) {
			line := fmt.Sprintf("%s (%s)", instance.Name, instance.State)
			if instance.ExternalIP != "" {
				line = fmt.Sprintf("%s %s", line, instance.ExternalIP)
			}
			if instance.InternalIP != "" {
				line = fmt.Sprintf("%s [%s]", line, instance.InternalIP)
			}
			state.UI.InfofIndent(1, "%s", line)
		}
		if entry.CreatedAt != "" {
			state.UI.InfofIndent(1, "Created: %s", entry.CreatedAt)
		}
		if entry.LastAction != "" {
			state.UI.InfofIndent(1, "Last action: %s (%s)", entry.LastAction, entry.LastActionAt)
		}
		if entry.DeletedAt != "" {
			state.UI.InfofIndent(1, "Deleted: %s", entry.DeletedAt)
		}
	}
	return nil
}

func stateRaw(c *cli.Context) error {
	state, err := GetState(c)
	if err != nil {
		return err
	}
	announce(state)

	data, err := state.State.Load()
	if err != nil {
		return err
	}
	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	fmt.Fprintln(state.UI.Out, string(raw))
	return nil
}

var _ = appstate.Data{}

func clusterConfigSummary(cfg appstate.ClusterConfig) string {
	items := []string{}
	if machineType := strings.TrimSpace(cfg.GCPMachineType); machineType != "" {
		items = append(items, fmt.Sprintf("machine=%s", machineType))
	}
	if cfg.GCPMaxRunHours > 0 {
		items = append(items, fmt.Sprintf("max-run-hours=%d", cfg.GCPMaxRunHours))
	}
	if action := strings.TrimSpace(cfg.GCPTerminationAction); action != "" {
		items = append(items, fmt.Sprintf("termination=%s", action))
	}
	if cfg.GCPDiskSizeGB > 0 {
		items = append(items, fmt.Sprintf("disk-size-gb=%d", cfg.GCPDiskSizeGB))
	}
	if cfg.KeepDisks {
		items = append(items, "keep-disks=true")
	}
	return strings.Join(items, ", ")
}
