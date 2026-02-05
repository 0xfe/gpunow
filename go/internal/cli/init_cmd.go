package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/urfave/cli/v2"

	"gpunow/internal/home"
	"gpunow/internal/ui"
)

func initCommand() *cli.Command {
	return &cli.Command{
		Name:  "init",
		Usage: "Initialize profiles in the user home directory",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "source",
				Usage: "Source profile directory (defaults to ./profiles/default or ../profiles/default)",
			},
			&cli.BoolFlag{
				Name:  "force",
				Usage: "Overwrite existing default profile",
			},
		},
		Action: initAction,
	}
}

func initAction(c *cli.Context) error {
	uiPrinter := ui.New()

	root, err := resolveInitHome()
	if err != nil {
		return err
	}
	profilesDir := filepath.Join(root, "profiles")
	targetProfile := filepath.Join(profilesDir, "default")
	stateDir := filepath.Join(root, "state")

	source := c.String("source")
	if source == "" {
		source, err = findDefaultProfileSource()
		if err != nil {
			return err
		}
	}

	if err := os.MkdirAll(profilesDir, 0o755); err != nil {
		return fmt.Errorf("create profiles dir: %w", err)
	}
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return fmt.Errorf("create state dir: %w", err)
	}

	if dirExists(targetProfile) {
		if !c.Bool("force") {
			return fmt.Errorf("profile already exists at %s (use --force to overwrite)", targetProfile)
		}
		if err := os.RemoveAll(targetProfile); err != nil {
			return fmt.Errorf("remove existing profile: %w", err)
		}
	}

	if err := copyDir(source, targetProfile); err != nil {
		return err
	}

	uiPrinter.Successf("Initialized profiles at %s", profilesDir)
	uiPrinter.Infof("Default profile: %s", targetProfile)
	uiPrinter.Infof("State dir: %s", stateDir)
	return nil
}

func resolveInitHome() (string, error) {
	if root := os.Getenv("GPUNOW_HOME"); root != "" {
		return filepath.Clean(root), nil
	}
	return home.DefaultRoot()
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
		if err := copyFile(srcPath, dstPath); err != nil {
			return err
		}
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open source file: %w", err)
	}
	defer in.Close()

	info, err := in.Stat()
	if err != nil {
		return fmt.Errorf("stat source file: %w", err)
	}

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode().Perm())
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
