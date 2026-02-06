package home

import (
	"fmt"
	"os"
	"path/filepath"
)

type Home struct {
	Root        string
	ProfilesDir string
	StateDir    string
	Source      string
}

func Resolve() (Home, error) {
	if root := os.Getenv("GPUNOW_HOME"); root != "" {
		return resolveFromRoot(root, "env")
	}

	configRoot, err := DefaultRoot()
	if err != nil {
		return Home{}, err
	}
	if dirExists(filepath.Join(configRoot, "profiles")) {
		return resolveFromRoot(configRoot, "config")
	}

	lookedAt := []string{}
	lookedAt = append(lookedAt, filepath.Join(configRoot, "profiles"))
	return Home{}, fmt.Errorf("profiles directory not found (looked in: %s). Set GPUNOW_HOME or create profiles/default.", joinPaths(lookedAt))
}

func resolveFromRoot(root, source string) (Home, error) {
	root = filepath.Clean(root)
	profilesDir := filepath.Join(root, "profiles")
	stateDir := filepath.Join(root, "state")

	if !dirExists(profilesDir) {
		return Home{}, fmt.Errorf("profiles directory not found: %s (set GPUNOW_HOME or create profiles/default)", profilesDir)
	}

	return Home{
		Root:        root,
		ProfilesDir: profilesDir,
		StateDir:    stateDir,
		Source:      source,
	}, nil
}

func DefaultRoot() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(homeDir, ".config", "gpunow"), nil
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

func joinPaths(paths []string) string {
	if len(paths) == 0 {
		return ""
	}
	out := paths[0]
	for i := 1; i < len(paths); i++ {
		out += ", " + paths[i]
	}
	return out
}
