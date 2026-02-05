package cli

import (
	"encoding/json"
	"fmt"
	"sort"

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
		state.UI.InfofIndent(1, "Instances: %d", entry.NumInstances)
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
