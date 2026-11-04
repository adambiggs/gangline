package main

import (
	"context"
	"errors"
	"path/filepath"

	"github.com/adambiggs/gangline/substrate"
)

type paneProcessReader interface {
	ForegroundCommand(context.Context, substrate.PaneID) (string, error)
	ProcessVisibility(context.Context, substrate.PaneID) (bool, error)
	PaneProcesses(context.Context, substrate.PaneID) ([]substrate.Process, error)
}

func (run *runtime) agentCommands() (map[string]bool, error) {
	names, err := collarNames(run.settings)
	if err != nil {
		return nil, err
	}
	commands := make(map[string]bool, len(names))
	for _, name := range names {
		c, err := loadCollar(name, run.settings)
		if err != nil {
			return nil, err
		}
		for name := range launchCommandNames(c.Launch.Command, resolveLaunchCommand) {
			commands[name] = true
		}
	}
	return commands, nil
}

func plainShell(command string) bool {
	switch filepath.Base(command) {
	case "sh", "bash", "dash", "zsh", "ksh", "fish", "csh", "tcsh":
		return true
	}
	return false
}

func unregisteredAgent(ctx context.Context, b paneProcessReader, pane substrate.PaneID, commands map[string]bool) (bool, error) {
	command, err := b.ForegroundCommand(ctx, pane)
	var exited *substrate.ExitedError
	if errors.As(err, &exited) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if commands[filepath.Base(command)] {
		return true, nil
	}
	unknown := func(cause error) (bool, error) {
		return false, errors.Join(refuseError("unregistered pane %s cannot be identified as a plain shell or configured harness; inspect it and close that exact pane or name its agent before hitching", pane), cause)
	}
	if !plainShell(command) {
		return unknown(nil)
	}
	visible, err := b.ProcessVisibility(ctx, pane)
	if errors.As(err, &exited) {
		return false, nil
	}
	if err != nil || !visible {
		// A foreground shell alone cannot exclude an old background harness.
		return unknown(err)
	}
	processes, err := b.PaneProcesses(ctx, pane)
	if errors.As(err, &exited) {
		return false, nil
	}
	if err != nil || len(processes) == 0 {
		return unknown(err)
	}
	for _, process := range processes {
		if commands[filepath.Base(process.Command)] || commands[filepath.Base(process.Name)] {
			return true, nil
		}
		if !plainShell(process.Name) || !plainShell(process.Command) {
			return unknown(nil)
		}
	}
	return false, nil
}
