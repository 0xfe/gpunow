package cloudinit

import (
	"fmt"
	"os"
	"strings"
)

const (
	setupPlaceholder = "{{SETUP_SH}}"
	zshrcPlaceholder = "{{ZSHRC}}"
)

func Render(templatePath, setupPath, zshrcPath string) (string, error) {
	tpl, err := os.ReadFile(templatePath)
	if err != nil {
		return "", fmt.Errorf("read cloud-init template: %w", err)
	}
	setup, err := os.ReadFile(setupPath)
	if err != nil {
		return "", fmt.Errorf("read setup script: %w", err)
	}
	zshrc, err := os.ReadFile(zshrcPath)
	if err != nil {
		return "", fmt.Errorf("read zshrc: %w", err)
	}

	lines := strings.Split(string(tpl), "\n")
	lines = replacePlaceholder(lines, setupPlaceholder, string(setup))
	lines = replacePlaceholder(lines, zshrcPlaceholder, string(zshrc))
	return strings.Join(lines, "\n"), nil
}

func replacePlaceholder(lines []string, placeholder string, content string) []string {
	if placeholder == "" {
		return lines
	}
	contentLines := strings.Split(content, "\n")
	var out []string
	for _, line := range lines {
		if !strings.Contains(line, placeholder) {
			out = append(out, line)
			continue
		}
		indent := line[:strings.Index(line, placeholder)]
		for _, contentLine := range contentLines {
			out = append(out, indent+contentLine)
		}
	}
	return out
}
