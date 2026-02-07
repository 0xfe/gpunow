package ssh

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func ResolvePublicKeySelection(configured string) (*PublicKeySelection, error) {
	configured = strings.TrimSpace(configured)
	if configured != "" {
		identity, err := ResolveIdentityFile(configured)
		if err != nil {
			return nil, err
		}
		pubPath := identity + ".pub"
		key, err := readPublicKey(pubPath)
		if err != nil {
			return nil, err
		}
		return &PublicKeySelection{
			Key:          key,
			Path:         pubPath,
			IdentityPath: identity,
			Source:       "identity_file",
		}, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	sshDir := filepath.Join(home, ".ssh")
	gpunowPath := filepath.Join(sshDir, "gpunow.pub")
	if key, err := readPublicKey(gpunowPath); err == nil {
		identity := resolveIdentityPath(gpunowPath)
		return &PublicKeySelection{
			Key:          key,
			Path:         gpunowPath,
			IdentityPath: identity,
			Source:       "gpunow.pub",
		}, nil
	} else if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	keys, err := listPublicKeys(sshDir)
	if err != nil {
		return nil, err
	}
	if len(keys) == 0 {
		return nil, fmt.Errorf("no SSH public keys found in %s\nGenerate one with:\n  ssh-keygen -t ed25519 -f %s",
			shortPath(sshDir),
			shortPath(filepath.Join(sshDir, "gpunow")),
		)
	}
	if len(keys) > 1 {
		lines := make([]string, 0, len(keys))
		for _, key := range keys {
			lines = append(lines, "  - "+shortPath(key))
		}
		return nil, fmt.Errorf("multiple SSH public keys found in %s:\n%s\nSelect one by symlinking to %s:\n  ln -s %s %s",
			shortPath(sshDir),
			strings.Join(lines, "\n"),
			shortPath(gpunowPath),
			shortPath(keys[0]),
			shortPath(gpunowPath),
		)
	}

	keyPath := keys[0]
	key, err := readPublicKey(keyPath)
	if err != nil {
		return nil, err
	}
	identity := resolveIdentityPath(keyPath)
	return &PublicKeySelection{
		Key:          key,
		Path:         keyPath,
		IdentityPath: identity,
		Source:       "fallback",
		Notice:       fmt.Sprintf("No %s found; using %s as default.", shortPath(gpunowPath), shortPath(keyPath)),
	}, nil
}

func listPublicKeys(sshDir string) ([]string, error) {
	entries, err := os.ReadDir(sshDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	keys := []string{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".pub") {
			continue
		}
		keys = append(keys, filepath.Join(sshDir, name))
	}
	sort.Strings(keys)
	return keys, nil
}

func readPublicKey(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	key := strings.TrimSpace(string(data))
	if key == "" {
		return "", fmt.Errorf("ssh public key is empty: %s", path)
	}
	canonical, err := canonicalizePublicKey(key)
	if err != nil {
		return "", fmt.Errorf("invalid SSH public key format: %s", path)
	}
	if canonical == "" {
		return "", err
	}
	return canonical, nil
}

// canonicalizePublicKey normalizes to "<keytype> <base64>", dropping comments.
func canonicalizePublicKey(key string) (string, error) {
	parts := strings.Fields(strings.TrimSpace(key))
	if len(parts) < 2 {
		return "", fmt.Errorf("invalid SSH public key")
	}
	return parts[0] + " " + parts[1], nil
}

func resolveIdentityPath(publicPath string) string {
	resolved := publicPath
	if real, err := filepath.EvalSymlinks(publicPath); err == nil {
		resolved = real
	}
	if !strings.HasSuffix(resolved, ".pub") {
		return ""
	}
	identity := strings.TrimSuffix(resolved, ".pub")
	if identity == "" {
		return ""
	}
	if _, err := os.Stat(identity); err == nil {
		return identity
	}
	return ""
}

func shortPath(path string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return path
	}
	path = filepath.Clean(path)
	home = filepath.Clean(home)
	if path == home {
		return "~"
	}
	if strings.HasPrefix(path, home+string(filepath.Separator)) {
		return "~" + string(filepath.Separator) + strings.TrimPrefix(path, home+string(filepath.Separator))
	}
	return path
}
