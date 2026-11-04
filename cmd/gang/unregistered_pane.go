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
		commands[filepath.Base(c.Launch.Command)] = true
	}
	return commands, nil
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
	visible, err := b.ProcessVisibility(ctx, pane)
	if errors.As(err, &exited) {
		return false, nil
	}
	if err != nil || !visible {
		// tmux's foreground observation remains available in a private PID
		// namespace; background descendants cannot be read from there.
		return false, err
	}
	processes, err := b.PaneProcesses(ctx, pane)
	if errors.As(err, &exited) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	for _, process := range processes {
		if commands[filepath.Base(process.Command)] || commands[filepath.Base(process.Name)] {
			return true, nil
		}
	}
	return false, nil
}
