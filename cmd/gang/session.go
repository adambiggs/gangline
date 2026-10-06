package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/adambiggs/gangline/store"
	"github.com/adambiggs/gangline/substrate"
)

func (cmd command) attach(arguments []string) error {
	if len(arguments) > 1 {
		return usageError("attach: expected at most one agent name")
	}
	if len(arguments) == 1 {
		if err := validateAgentName(arguments[0]); err != nil {
			return err
		}
	}
	run, err := cmd.runtime()
	if err != nil {
		return err
	}
	backend, err := cmd.tmux(run.settings)
	if err != nil {
		return err
	}
	exists, err := backend.SessionExists(context.Background())
	if err != nil {
		return err
	}
	if !exists {
		return run.stoppedTeamError()
	}
	agents, err := run.team.ListAgents()
	if err != nil {
		return err
	}
	if len(agents) == 0 {
		return refuseError("team %q has no registered agents", run.settings.Session)
	}
	name := string(agents[0].Name)
	for _, agent := range agents {
		if agent.Role == "lead" {
			name = string(agent.Name)
			break
		}
	}
	if len(arguments) == 1 {
		name = arguments[0]
	}
	agent, err := run.resolve(name)
	if err != nil {
		return err
	}
	if err := requirePaneRegistration(agent); err != nil {
		return err
	}
	present, err := backend.CheckPane(context.Background(), paneIdentity(agent))
	if err != nil {
		return err
	}
	if !present {
		return refuseError("%s has no running pane", agent.Name)
	}
	return backend.Attach(context.Background(), substrate.PaneID(agent.Pane))
}

func (run *runtime) stoppedTeamError() error {
	return refuseError("no team %q is running; start it with 'gang up'", run.settings.Session)
}

func (cmd command) teams(arguments []string) error {
	if err := noArguments(arguments, "teams"); err != nil {
		return err
	}
	settings, err := cmd.settings()
	if err != nil {
		return err
	}
	directory := filepath.Join(settings.StateRoot, "teams")
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
		if _, err := os.Stat(paths.State); err != nil {
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
	var yes bool
	flags := boundFlagSet("upgrade", map[string]any{"y": &yes, "yes": &yes})
	positionals, err := parseOptions(flags, arguments)
	if err != nil {
		return usageError("upgrade: %v", err)
	}
	if len(positionals) != 0 {
		return usageError("upgrade: unexpected argument %q", positionals[0])
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
	script, err := os.ReadFile(installer)
	if err != nil {
		return fmt.Errorf("upgrade: %w", err)
	}
	if !bytes.Contains(script, []byte("GANGLINE_UPGRADE_CONFIRM")) {
		// An installer that ignores the confirmation mode would install
		// without asking.
		return refuseError("upgrade: the installer at %s cannot confirm an upgrade; reinstall with install.sh; no changes made", installRoot)
	}
	confirm := "yes"
	if !yes {
		confirm = "refuse"
		if cmd.stdinIsTerminal() {
			confirm = "ask"
		}
	}
	process := exec.Command("sh", installer)
	process.Stdin = cmd.stdin
	process.Stdout = childOutput(cmd.stdout)
	process.Stderr = childOutput(cmd.stderr)
	process.Env = append(os.Environ(), "GANGLINE_UPGRADE=1", "GANGLINE_UPGRADE_CONFIRM="+confirm, "GANGLINE_HOME="+installRoot)
	err = process.Run()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == exitRefused {
		// The installer exits 3 only after it has named both versions and
		// the confirmation it was given did not allow the install.
		switch confirm {
		case "refuse":
			return refuseError("upgrade requires a terminal for confirmation; use --yes to install non-interactively; no changes made")
		case "ask":
			return refuseError("upgrade cancelled; no changes made")
		}
	}
	if err != nil {
		return fmt.Errorf("upgrade: %w", err)
	}
	return nil
}
