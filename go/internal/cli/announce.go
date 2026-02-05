package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"gpunow/internal/version"
)

func announce(state *State) {
	cfg := state.Config
	profile := cfg.Profile
	if profile == "" {
		profile = "default"
	}

	configPath := cfg.Paths.Dir
	if wd, err := os.Getwd(); err == nil {
		if rel, err := filepath.Rel(wd, configPath); err == nil {
			configPath = rel
		}
	}

	state.UI.Infof("gpunow %s", version.String())
	state.UI.Infof("profile: %s (%s) | project: %s | zone: %s", profile, configPath, cfg.Project.ID, cfg.Project.Zone)
	fmt.Fprintln(state.UI.Out)
}
