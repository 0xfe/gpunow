package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gpunow/internal/ssh"
	"gpunow/internal/version"
)

func announce(state *State) {
	announceWithKey(state, nil, false)
}

func announceWithKey(state *State, selection *ssh.PublicKeySelection, showNotice bool) {
	cfg := state.Config
	profile := cfg.Profile
	if profile == "" {
		profile = "default"
	}

	configPath := cfg.Paths.Dir
	if state.Home.Root != "" {
		if rel, err := filepath.Rel(state.Home.Root, configPath); err == nil && !strings.HasPrefix(rel, "..") {
			configPath = rel
		}
	}
	if wd, err := os.Getwd(); err == nil {
		if rel, err := filepath.Rel(wd, configPath); err == nil {
			configPath = rel
		}
	}

	state.UI.Infof("gpunow %s", version.String())
	if state.Home.Root != "" {
		state.UI.Infof("home: %s (source: %s)", state.Home.Root, state.Home.Source)
	}
	state.UI.Infof("profile: %s (%s) | project: %s | zone: %s", profile, configPath, cfg.Project.ID, cfg.Project.Zone)
	if selection != nil && selection.Path != "" {
		state.UI.Infof("ssh key: %s", selection.Path)
		if showNotice && selection.Notice != "" {
			state.UI.Infof("%s", selection.Notice)
		}
	}
	fmt.Fprintln(state.UI.Out)
}

func announceStatus(state *State) {
	announceStatusWithKey(state, nil, false)
}

func announceStatusWithKey(state *State, selection *ssh.PublicKeySelection, showNotice bool) {
	cfg := state.Config

	state.UI.Infof("gpunow %s", version.String())
	if state.Home.Root != "" {
		state.UI.Infof("home: %s (source: %s)", state.Home.Root, state.Home.Source)
	}
	state.UI.Infof("project: %s | zone: %s", cfg.Project.ID, cfg.Project.Zone)
	if selection != nil && selection.Path != "" {
		state.UI.Infof("ssh key: %s", selection.Path)
		if showNotice && selection.Notice != "" {
			state.UI.Infof("%s", selection.Notice)
		}
	}
	fmt.Fprintln(state.UI.Out)
}
