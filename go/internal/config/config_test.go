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

func TestLoadConfigDefaultsHostnameAndOptionalServiceAccount(t *testing.T) {
	baseConfigPath := filepath.Join("..", "..", "..", "profiles", "default", "config.toml")
	data, err := os.ReadFile(baseConfigPath)
	if err != nil {
		t.Fatalf("read default config: %v", err)
	}
	updated := string(data)
	updated = removeTOMLSection(updated, "service_account")
	updated = strings.ReplaceAll(updated, "hostname_domain = \"gpunow\"\n", "")

	tmp := t.TempDir()
	configDir := filepath.Join(tmp, "defaults")
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
	if err := os.WriteFile(filepath.Join(configDir, "zshrc"), []byte("export TEST=1\n"), 0o644); err != nil {
		t.Fatalf("write zshrc: %v", err)
	}

	cfg, err := Load("defaults", tmp)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Instance.HostnameDomain != "gpunow" {
		t.Fatalf("hostname_domain default mismatch: got=%q", cfg.Instance.HostnameDomain)
	}
}

func removeTOMLSection(content, section string) string {
	lines := strings.Split(content, "\n")
	header := "[" + section + "]"
	start := -1
	end := len(lines)
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "[") || !strings.HasSuffix(trimmed, "]") {
			continue
		}
		if trimmed == header {
			start = i
			for j := i + 1; j < len(lines); j++ {
				next := strings.TrimSpace(lines[j])
				if strings.HasPrefix(next, "[") && strings.HasSuffix(next, "]") {
					end = j
					break
				}
			}
			break
		}
	}
	if start < 0 {
		return content
	}
	return strings.Join(append(lines[:start], lines[end:]...), "\n")
}
