package cloudinit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRender(t *testing.T) {
	tmp := t.TempDir()
	tplPath := filepath.Join(tmp, "cloud-init.yaml")
	setupPath := filepath.Join(tmp, "setup.sh")
	zshrcPath := filepath.Join(tmp, "zshrc")

	tpl := "write_files:\n  - content: |\n      {{SETUP_SH}}\n  - content: |\n      {{ZSHRC}}\n"
	setup := "echo one\necho two\n"
	zshrc := "export PATH=$PATH\n"

	if err := os.WriteFile(tplPath, []byte(tpl), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}
	if err := os.WriteFile(setupPath, []byte(setup), 0o644); err != nil {
		t.Fatalf("write setup: %v", err)
	}
	if err := os.WriteFile(zshrcPath, []byte(zshrc), 0o644); err != nil {
		t.Fatalf("write zshrc: %v", err)
	}

	rendered, err := Render(tplPath, setupPath, zshrcPath)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(rendered, "      echo one") || !strings.Contains(rendered, "      echo two") {
		t.Fatalf("rendered content missing indented setup lines:\n%s", rendered)
	}
	if !strings.Contains(rendered, "      export PATH=$PATH") {
		t.Fatalf("rendered content missing zshrc lines:\n%s", rendered)
	}
}
