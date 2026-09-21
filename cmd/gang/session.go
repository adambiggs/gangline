package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/adambiggs/gangline/store"
)

func (cmd command) attach(arguments []string) error {
	if err := noArguments(arguments, "attach"); err != nil {
		return err
	}
	settings, err := cmd.settings()
	if err != nil {
		return err
	}
	backend, err := cmd.tmux(settings)
	if err != nil {
		return err
	}
	exists, err := backend.SessionExists(context.Background())
	if err != nil {
		return err
	}
	if !exists {
		return refuseError("no team %q is running; start it with 'gang up'", settings.Session)
	}
	windows, err := backend.Windows(context.Background())
	if err != nil {
		return err
	}
	if len(windows) == 0 {
		return fmt.Errorf("team %q has no panes", settings.Session)
	}
	return backend.Attach(context.Background(), windows[0].Pane.ID)
}

func (cmd command) teams(arguments []string) error {
	if err := noArguments(arguments, "teams"); err != nil {
		return err
	}
	settings, err := cmd.settings()
	if err != nil {
		return err
	}
	directory := filepath.Join(settings.StateRoot, store.LayoutVersion)
	entries, err := os.ReadDir(directory)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("list teams: %w", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		paths, err := (store.Paths{Root: settings.StateRoot}).Team(entry.Name())
		if err != nil {
			return fmt.Errorf("list teams: %w", err)
		}
		if _, err := os.Stat(paths.Events); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("inspect team %q: %w", entry.Name(), err)
		}
		if _, err := fmt.Fprintln(cmd.stdout, entry.Name()); err != nil {
			return err
		}
	}
	return nil
}

func (cmd command) upgrade(arguments []string) error {
	if len(arguments) > 1 || (len(arguments) == 1 && arguments[0] != "--check") {
		return usageError("upgrade accepts only --check")
	}
	home, err := cmd.userHomeDir()
	if err != nil {
		return fmt.Errorf("locate home directory: %w", err)
	}
	installRoot := valueOr(cmd.getenv("GANGLINE_HOME"), filepath.Join(home, ".local", "share", "gangline"))
	installer := filepath.Join(installRoot, "install.sh")
	if info, err := os.Stat(installer); err != nil || info.IsDir() {
		return fmt.Errorf("upgrade requires an installer-managed release at %s", installRoot)
	}
	processArgs := []string{installer}
	if len(arguments) == 1 {
		processArgs = append(processArgs, "--check")
	}
	process := exec.Command("sh", processArgs...)
	process.Stdin = cmd.stdin
	process.Stdout = cmd.stdout
	process.Stderr = cmd.stderr
	process.Env = append(os.Environ(), "GANGLINE_UPGRADE=1", "GANGLINE_HOME="+installRoot)
	if err := process.Run(); err != nil {
		return fmt.Errorf("upgrade: %w", err)
	}
	return nil
}
