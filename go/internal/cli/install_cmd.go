package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/urfave/cli/v2"

	"gpunow/internal/home"
	"gpunow/internal/ui"
)

func installCommand() *cli.Command {
	return &cli.Command{
		Name:  "install",
		Usage: "Install gpunow binary and default profile in user home",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "source",
				Usage: "Source profile directory (defaults to ./profiles/default or ../profiles/default)",
			},
			&cli.BoolFlag{
				Name:  "overwrite",
				Usage: "Overwrite existing ~/.config/gpunow",
			},
		},
		Action: installAction,
	}
}

func installAction(c *cli.Context) error {
	uiPrinter := ui.New()

	root, err := home.DefaultRoot()
	if err != nil {
		return err
	}

	if dirExists(root) && !c.Bool("overwrite") {
		return fmt.Errorf("config already exists at %s (use --overwrite to replace)", root)
	}

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve binary path: %w", err)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home directory: %w", err)
	}
	binDir := filepath.Join(homeDir, ".local", "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return fmt.Errorf("create bin dir: %w", err)
	}
	targetBin := filepath.Join(binDir, "gpunow")
	if err := copyFileWithMode(exe, targetBin, 0o755); err != nil {
		return err
	}

	profilesDir := filepath.Join(root, "profiles")
	stateDir := filepath.Join(root, "state")

	source := c.String("source")
	if source == "" {
		source, err = findDefaultProfileSource()
		if err != nil {
			return err
		}
	}

	if c.Bool("overwrite") && dirExists(profilesDir) {
		timestamp := time.Now().UTC().Format("20060102150405")
		backupDir := filepath.Join(root, fmt.Sprintf("profiles.old.%s", timestamp))
		if err := os.Rename(profilesDir, backupDir); err != nil {
			return fmt.Errorf("move existing profiles: %w", err)
		}
		uiPrinter.Infof("Moved existing profiles to %s", backupDir)
	}

	if err := os.MkdirAll(profilesDir, 0o755); err != nil {
		return fmt.Errorf("create profiles dir: %w", err)
	}
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return fmt.Errorf("create state dir: %w", err)
	}

	targetProfile := filepath.Join(profilesDir, "default")
	if err := copyDir(source, targetProfile); err != nil {
		return err
	}

	uiPrinter.Successf("Installed %s", targetBin)
	uiPrinter.Successf("Initialized profiles at %s", profilesDir)
	uiPrinter.Infof("Default profile: %s", targetProfile)
	uiPrinter.Infof("State dir: %s", stateDir)
	uiPrinter.Heading("Shell setup")
	uiPrinter.Infof("Add to ~/.zshrc or ~/.bashrc:")
	uiPrinter.Dimf("export PATH=\"$HOME/.local/bin:$PATH\"")
	uiPrinter.Dimf("export GPUNOW_HOME=\"%s\"", root)
	return nil
}

func findDefaultProfileSource() (string, error) {
	if cwd, err := os.Getwd(); err == nil {
		path := filepath.Join(cwd, "profiles", "default")
		if dirExists(path) {
			return path, nil
		}
	}
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		paths := []string{
			filepath.Join(exeDir, "profiles", "default"),
			filepath.Join(exeDir, "..", "profiles", "default"),
			filepath.Join(exeDir, "..", "..", "profiles", "default"),
		}
		for _, path := range paths {
			if dirExists(path) {
				return path, nil
			}
		}
	}
	return "", fmt.Errorf("default profile source not found (run from repo or pass --source)")
}

func copyDir(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("source profile not found: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("source profile is not a directory: %s", src)
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return fmt.Errorf("create target profile: %w", err)
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("read source profile: %w", err)
	}
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
			continue
		}
		if err := copyFileWithMode(srcPath, dstPath, 0); err != nil {
			return err
		}
	}
	return nil
}

func copyFileWithMode(src, dst string, forceMode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open source file: %w", err)
	}
	defer in.Close()

	info, err := in.Stat()
	if err != nil {
		return fmt.Errorf("stat source file: %w", err)
	}
	mode := info.Mode().Perm()
	if forceMode != 0 {
		mode = forceMode
	}

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return fmt.Errorf("open target file: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("copy file: %w", err)
	}
	return nil
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}
