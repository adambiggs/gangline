package main

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/substrate"
)

type harnessInput interface {
	ForegroundProcesses(context.Context, substrate.PaneID) ([]substrate.Process, error)
	SendKeys(context.Context, substrate.PaneID, substrate.Keys) error
}

func sendHarnessKeys(ctx context.Context, backend harnessInput, pane substrate.PaneID, collar harness.Collar, keys substrate.Keys) error {
	if err := requireHarnessForeground(ctx, backend, pane, collar); err != nil {
		return err
	}
	return backend.SendKeys(ctx, pane, keys)
}

func requireHarnessForeground(ctx context.Context, backend harnessInput, pane substrate.PaneID, collar harness.Collar) error {
	processes, err := backend.ForegroundProcesses(ctx, pane)
	if err != nil {
		return fmt.Errorf("refuse input without foreground-process evidence: %w", err)
	}
	want := filepath.Base(collar.Launch.Command)
	for _, process := range processes {
		if filepath.Base(process.Command) == want {
			return nil
		}
	}
	commands := make([]string, len(processes))
	for index, process := range processes {
		commands[index] = process.Command
	}
	return fmt.Errorf("refuse input: pane foreground is %q, want harness %q", strings.Join(commands, ","), want)
}
