package ssh

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type PublicKeySelection struct {
	Key          string
	Path         string
	IdentityPath string
	Source       string
	Notice       string
}

func ResolveIdentityFile(configured string) (string, error) {
	configured = strings.TrimSpace(configured)
	if configured != "" {
		expanded, err := ExpandPath(configured)
		if err != nil {
			return "", err
		}
		if _, err := os.Stat(expanded); err != nil {
			if os.IsNotExist(err) {
				return "", fmt.Errorf("ssh identity file not found: %s", expanded)
			}
			return "", err
		}
		return expanded, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	defaultPath := filepath.Join(home, ".ssh", "google_compute_engine")
	if _, err := os.Stat(defaultPath); err == nil {
		return defaultPath, nil
	}
	return "", nil
}

func ResolvePublicKey(configured string) (string, error) {
	selection, err := ResolvePublicKeySelection(configured)
	if err != nil {
		return "", err
	}
	if selection == nil {
		return "", nil
	}
	return selection.Key, nil
}

func ExpandPath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", nil
	}
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		if path == "~" {
			return home, nil
		}
		return filepath.Join(home, path[2:]), nil
	}
	return path, nil
}
