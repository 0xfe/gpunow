package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/urfave/cli/v2"
)

func configCommand() *cli.Command {
	return &cli.Command{
		Name:  "config",
		Usage: "Show or update profile config",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "gcp-project-id", Usage: "Set [project].id in config.toml"},
			&cli.StringFlag{Name: "gcp-zone", Usage: "Set [project].zone in config.toml"},
			&cli.StringFlag{Name: "gcp-machine-type", Usage: "Set [instance].machine_type in config.toml"},
			&cli.IntFlag{Name: "gcp-max-run-hours", Usage: "Set [instance].max_run_hours in config.toml"},
			&cli.StringFlag{Name: "gcp-termination-action", Usage: "Set [instance].termination_action in config.toml (DELETE|STOP)"},
			&cli.IntFlag{Name: "gcp-disk-size-gb", Usage: "Set [disk].size_gb in config.toml"},
		},
		Action: configAction,
	}
}

func configAction(c *cli.Context) error {
	state, err := GetState(c)
	if err != nil {
		return err
	}

	projectID := strings.TrimSpace(c.String("gcp-project-id"))
	zone := strings.TrimSpace(c.String("gcp-zone"))
	machineType := strings.TrimSpace(c.String("gcp-machine-type"))
	terminationAction := strings.ToUpper(strings.TrimSpace(c.String("gcp-termination-action")))
	maxRunHours := c.Int("gcp-max-run-hours")
	diskSizeGB := c.Int("gcp-disk-size-gb")
	hasUpdates := c.IsSet("gcp-project-id") || c.IsSet("gcp-zone") || c.IsSet("gcp-machine-type") || c.IsSet("gcp-max-run-hours") || c.IsSet("gcp-termination-action") || c.IsSet("gcp-disk-size-gb")
	if !hasUpdates {
		announce(state)
		state.UI.Heading("Config")
		state.UI.Infof("File: %s", state.Config.Paths.ConfigFile)
		state.UI.Infof("gcp-project-id: %s", state.Config.Project.ID)
		state.UI.Infof("gcp-zone: %s", state.Config.Project.Zone)
		state.UI.Infof("gcp-machine-type: %s", state.Config.Instance.MachineType)
		state.UI.Infof("gcp-max-run-hours: %d", state.Config.Instance.MaxRunHours)
		state.UI.Infof("gcp-termination-action: %s", state.Config.Instance.TerminationAction)
		state.UI.Infof("gcp-disk-size-gb: %d", state.Config.Disk.SizeGB)
		return nil
	}
	if c.IsSet("gcp-project-id") && projectID == "" {
		return usageError(c, "--gcp-project-id cannot be empty")
	}
	if c.IsSet("gcp-zone") && zone == "" {
		return usageError(c, "--gcp-zone cannot be empty")
	}
	if c.IsSet("gcp-machine-type") && machineType == "" {
		return usageError(c, "--gcp-machine-type cannot be empty")
	}
	if c.IsSet("gcp-max-run-hours") && maxRunHours <= 0 {
		return usageError(c, "--gcp-max-run-hours must be a positive integer")
	}
	if c.IsSet("gcp-termination-action") && terminationAction != "DELETE" && terminationAction != "STOP" {
		return usageError(c, "--gcp-termination-action must be DELETE or STOP")
	}
	if c.IsSet("gcp-disk-size-gb") && diskSizeGB <= 0 {
		return usageError(c, "--gcp-disk-size-gb must be a positive integer")
	}

	raw, err := os.ReadFile(state.Config.Paths.ConfigFile)
	if err != nil {
		return fmt.Errorf("read config.toml: %w", err)
	}
	content := string(raw)
	if projectID != "" {
		content, err = setTOMLStringKey(content, "project", "id", projectID)
		if err != nil {
			return err
		}
	}
	if zone != "" {
		content, err = setTOMLStringKey(content, "project", "zone", zone)
		if err != nil {
			return err
		}
	}
	if machineType != "" {
		content, err = setTOMLStringKey(content, "instance", "machine_type", machineType)
		if err != nil {
			return err
		}
	}
	if c.IsSet("gcp-max-run-hours") {
		content, err = setTOMLIntKey(content, "instance", "max_run_hours", maxRunHours)
		if err != nil {
			return err
		}
	}
	if c.IsSet("gcp-termination-action") {
		content, err = setTOMLStringKey(content, "instance", "termination_action", terminationAction)
		if err != nil {
			return err
		}
	}
	if c.IsSet("gcp-disk-size-gb") {
		content, err = setTOMLIntKey(content, "disk", "size_gb", diskSizeGB)
		if err != nil {
			return err
		}
	}

	info, err := os.Stat(state.Config.Paths.ConfigFile)
	if err != nil {
		return fmt.Errorf("stat config.toml: %w", err)
	}
	tmpPath := state.Config.Paths.ConfigFile + ".tmp"
	if err := os.WriteFile(tmpPath, []byte(content), info.Mode().Perm()); err != nil {
		return fmt.Errorf("write config.toml: %w", err)
	}
	if err := os.Rename(tmpPath, state.Config.Paths.ConfigFile); err != nil {
		return fmt.Errorf("replace config.toml: %w", err)
	}

	announce(state)
	state.UI.Successf("Updated %s", state.Config.Paths.ConfigFile)
	if projectID != "" {
		state.UI.Infof("gcp-project-id: %s", projectID)
	}
	if zone != "" {
		state.UI.Infof("gcp-zone: %s", zone)
	}
	if machineType != "" {
		state.UI.Infof("gcp-machine-type: %s", machineType)
	}
	if c.IsSet("gcp-max-run-hours") {
		state.UI.Infof("gcp-max-run-hours: %d", maxRunHours)
	}
	if c.IsSet("gcp-termination-action") {
		state.UI.Infof("gcp-termination-action: %s", terminationAction)
	}
	if c.IsSet("gcp-disk-size-gb") {
		state.UI.Infof("gcp-disk-size-gb: %d", diskSizeGB)
	}
	state.UI.Infof("Run commands in a new invocation so updated config values are reloaded.")
	return nil
}

func setTOMLStringKey(content, section, key, value string) (string, error) {
	return setTOMLKey(content, section, key, fmt.Sprintf("%s = %q", key, value))
}

func setTOMLIntKey(content, section, key string, value int) (string, error) {
	return setTOMLKey(content, section, key, fmt.Sprintf("%s = %d", key, value))
}

func setTOMLKey(content, section, key, renderedLine string) (string, error) {
	lines := strings.Split(content, "\n")
	sectionHeader := "[" + section + "]"

	sectionStart := -1
	sectionEnd := len(lines)
	for idx, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !isTOMLSection(trimmed) {
			continue
		}
		if trimmed == sectionHeader {
			sectionStart = idx
			for j := idx + 1; j < len(lines); j++ {
				if isTOMLSection(strings.TrimSpace(lines[j])) {
					sectionEnd = j
					break
				}
			}
			break
		}
	}
	if sectionStart < 0 {
		return "", fmt.Errorf("section [%s] not found in config.toml", section)
	}

	for idx := sectionStart + 1; idx < sectionEnd; idx++ {
		trimmed := strings.TrimSpace(lines[idx])
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		eq := strings.Index(trimmed, "=")
		if eq <= 0 {
			continue
		}
		if strings.TrimSpace(trimmed[:eq]) == key {
			lines[idx] = renderedLine
			return strings.Join(lines, "\n"), nil
		}
	}

	lines = append(lines[:sectionEnd], append([]string{renderedLine}, lines[sectionEnd:]...)...)
	return strings.Join(lines, "\n"), nil
}

func isTOMLSection(line string) bool {
	return strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]")
}
