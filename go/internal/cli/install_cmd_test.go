package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCopyDir(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "src")
	dst := filepath.Join(tmp, "dst")

	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatalf("mkdir src: %v", err)
	}
	if err := os.WriteFile(filepath.Join(src, "config.toml"), []byte("ok"), 0o644); err != nil {
		t.Fatalf("write src: %v", err)
	}

	if err := copyDir(src, dst); err != nil {
		t.Fatalf("copy dir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, "config.toml")); err != nil {
		t.Fatalf("missing file: %v", err)
	}
}
