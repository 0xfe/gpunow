package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadDefaultConfig(t *testing.T) {
	base := filepath.Join("..", "..", "..", "profiles")
	cfg, err := Load("default", base)
	if err != nil {
		t.Fatalf("load default config: %v", err)
	}
	if _, err := os.Stat(cfg.Paths.CloudInitFile); err != nil {
		t.Fatalf("cloud-init file missing: %v", err)
	}
	if _, err := os.Stat(cfg.Paths.SetupScript); err != nil {
		t.Fatalf("setup script missing: %v", err)
	}
}

func TestLoadInvalidConfig(t *testing.T) {
	tmp := t.TempDir()
	configDir := filepath.Join(tmp, "bad")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	configPath := filepath.Join(configDir, "config.toml")
	if err := os.WriteFile(configPath, []byte("[project]\nid=\"\"\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := Load("bad", tmp)
	if err == nil {
		t.Fatalf("expected error for invalid config")
	}
}

func TestInvalidHostnameDomain(t *testing.T) {
	baseConfigPath := filepath.Join("..", "..", "..", "profiles", "default", "config.toml")
	data, err := os.ReadFile(baseConfigPath)
	if err != nil {
		t.Fatalf("read default config: %v", err)
	}
	updated := strings.Replace(string(data), "[instance]\n", "[instance]\nhostname_domain = \"bad\"\n", 1)

	tmp := t.TempDir()
	configDir := filepath.Join(tmp, "bad-hostname")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte(updated), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "cloud-init.yaml"), []byte("#cloud-config\n"), 0o644); err != nil {
		t.Fatalf("write cloud-init: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "setup.sh"), []byte("#!/bin/bash\n"), 0o644); err != nil {
		t.Fatalf("write setup: %v", err)
	}

	if _, err := Load("bad-hostname", tmp); err == nil {
		t.Fatalf("expected error for invalid hostname_domain")
	}
}
