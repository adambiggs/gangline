package main

import (
	"context"
	"fmt"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/substrate"
)

func (cmd command) context(arguments []string) error {
	name, state, run, err := cmd.observationTarget(arguments, "context")
	if err != nil {
		return err
	}
	hitch, _ := activeByName(state, name)
	backend, err := cmd.tmux(run.settings)
	if err != nil {
		return err
	}
	screen, err := backend.Capture(context.Background(), substrate.PaneID(hitch.Pane))
	if err != nil {
		return err
	}
	collar, err := loadCollar(hitch.Collar, run.settings)
	if err != nil {
		return err
	}
	reading, err := harness.ReadContext(collar.Primitives.Context, screen)
	if err != nil {
		return commandError{status: exitUnknown, text: err.Error()}
	}
	model := ""
	if collar.Models.Selected != nil {
		model, _ = harness.ReadSelectedModel(*collar.Models.Selected, screen)
	}
	band := harness.ActiveContextBand(collar, model, reading)
	bandName := "none"
	if band != nil {
		bandName = band.Name
	}
	_, err = fmt.Fprintf(cmd.stdout, "%s\t%d/%d\t%.0f%%\t%s\n", name, reading.Used, reading.Limit, reading.Percent*100, bandName)
	return err
}

func (cmd command) limits(arguments []string) error {
	name, state, run, err := cmd.observationTarget(arguments, "limits")
	if err != nil {
		return err
	}
	hitch, _ := activeByName(state, name)
	backend, err := cmd.tmux(run.settings)
	if err != nil {
		return err
	}
	screen, err := backend.Capture(context.Background(), substrate.PaneID(hitch.Pane))
	if err != nil {
		return err
	}
	collar, err := loadCollar(hitch.Collar, run.settings)
	if err != nil {
		return err
	}
	readings, err := harness.ReadProviderLimits(collar.Primitives.ProviderLimits, screen, time.Now())
	if err != nil {
		return commandError{status: exitUnknown, text: err.Error()}
	}
	for _, reading := range readings {
		if _, err := fmt.Fprintf(cmd.stdout, "%s\t%d%%\t%s\n", reading.Label, reading.UsedPercent, reading.ResetAt.Format(time.RFC3339)); err != nil {
			return err
		}
	}
	return nil
}

func (cmd command) observationTarget(arguments []string, commandName string) (string, core.State, *runtime, error) {
	if len(arguments) > 1 {
		return "", core.State{}, nil, usageError("%s: expected at most one agent", commandName)
	}
	name := ""
	if len(arguments) == 1 {
		name = arguments[0]
		if err := validateAgentName(name); err != nil {
			return "", core.State{}, nil, err
		}
	}
	run, err := cmd.runtime()
	if err != nil {
		return "", core.State{}, nil, err
	}
	state, err := run.load()
	if err != nil {
		return "", core.State{}, nil, err
	}
	if name == "" {
		pane := cmd.environment("TMUX_PANE")
		for _, hitch := range state.Hitches {
			if hitch.Pane == pane {
				name = string(hitch.Name)
				break
			}
		}
	}
	if _, ok := activeByName(state, name); !ok {
		return "", core.State{}, nil, refuseError("agent %q is not active", name)
	}
	return name, state, run, nil
}
