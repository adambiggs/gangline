package main

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/substrate"
)

func (cmd command) context(arguments []string) error {
	name, err := observationName(arguments, "context")
	if err != nil {
		return err
	}
	run, state, err := cmd.loaded()
	if err != nil {
		return err
	}
	hitch, err := cmd.observationTarget(name, state)
	if err != nil {
		return err
	}
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
	_, err = fmt.Fprintf(cmd.stdout, "%s\t%d/%d\t%.0f%%\t%s\n", hitch.Name, reading.Used, reading.Limit, reading.Percent*100, bandName)
	return err
}

func (cmd command) limits(arguments []string) error {
	name, err := observationName(arguments, "limits")
	if err != nil {
		return err
	}
	run, state, err := cmd.loaded()
	if err != nil {
		return err
	}
	hitch, err := cmd.observationTarget(name, state)
	if err != nil {
		return err
	}
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

func observationName(arguments []string, commandName string) (string, error) {
	if len(arguments) > 1 {
		return "", usageError("%s: expected at most one agent", commandName)
	}
	name := ""
	if len(arguments) == 1 {
		name = arguments[0]
		if err := validateAgentName(name); err != nil {
			return "", err
		}
	}
	return name, nil
}

func (cmd command) observationTarget(name string, state core.State) (core.Hitch, error) {
	if name == "" {
		name = nameAtPane(state, cmd.environment("TMUX_PANE"))
	}
	hitch, ok := activeByName(state, name)
	if !ok {
		return core.Hitch{}, refuseError("agent %q is not active", name)
	}
	return hitch, nil
}

func (cmd command) capture(arguments []string) error {
	composer := false
	if len(arguments) != 0 && arguments[0] == "--composer" {
		composer = true
		arguments = arguments[1:]
	}
	if len(arguments) > 2 {
		return usageError("capture: too many arguments")
	}
	name := ""
	if len(arguments) >= 1 {
		name = arguments[0]
		if err := validateAgentName(name); err != nil {
			return err
		}
	}
	lineCount := 0
	if len(arguments) == 2 {
		if composer {
			return usageError("capture --composer does not accept a line count")
		}
		parsed, err := strconv.Atoi(arguments[1])
		if err != nil || parsed <= 0 {
			return usageError("capture: lines must be a positive whole number")
		}
		lineCount = parsed
	}
	if composer {
		run, state, err := cmd.loaded()
		if err != nil {
			return err
		}
		if name == "" {
			name = nameAtPane(state, cmd.environment("TMUX_PANE"))
		}
		hitch, ok := activeByName(state, name)
		if !ok {
			return refuseError("agent %q is not active", name)
		}
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
		reading, err := harness.ReadComposer(collar.Primitives.Composer, screen)
		if err != nil {
			return commandError{status: exitUnknown, text: err.Error()}
		}
		_, err = fmt.Fprintln(cmd.stdout, reading.Text)
		return err
	}

	run, state, err := cmd.loaded()
	if err != nil {
		return err
	}
	backend, err := cmd.tmux(run.settings)
	if err != nil {
		return err
	}
	var pane substrate.Pane
	if name == "" {
		paneID := cmd.environment("TMUX_PANE")
		if paneID == "" {
			return refuseError("capture without an agent name must run inside tmux")
		}
		pane = substrate.Pane{ID: substrate.PaneID(paneID)}
	} else {
		hitch, targetErr := cmd.observationTarget(name, state)
		if targetErr != nil {
			return targetErr
		}
		pane = substrate.Pane{ID: substrate.PaneID(hitch.Pane)}
	}
	screen, err := backend.Capture(context.Background(), pane.ID)
	if err != nil {
		return err
	}
	text := screen.Text(lineCount)
	if text == "" {
		return nil
	}
	_, err = fmt.Fprintln(cmd.stdout, text)
	return err
}
