package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/substrate"
)

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
		settings, err := cmd.settings()
		if err != nil {
			return err
		}
		run := &runtime{cmd: cmd, settings: settings}
		state, err := run.load()
		if err != nil {
			return err
		}
		if name == "" {
			paneID := cmd.environment("TMUX_PANE")
			for _, hitch := range state.Hitches {
				if hitch.Pane == paneID {
					name = string(hitch.Name)
					break
				}
			}
		}
		hitch, ok := activeByName(state, name)
		if !ok {
			return refuseError("agent %q is not active", name)
		}
		backend, err := cmd.tmux(settings)
		if err != nil {
			return err
		}
		screen, err := backend.Capture(context.Background(), substrate.PaneID(hitch.Pane))
		if err != nil {
			return err
		}
		collar, err := loadCollar(hitch.Collar, settings)
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

	settings, err := cmd.settings()
	if err != nil {
		return err
	}
	backend, err := cmd.tmux(settings)
	if err != nil {
		return err
	}
	var pane substrate.Pane
	if name == "" {
		pane, err = backend.CurrentPane(context.Background())
	} else {
		pane, err = backend.PaneNamed(context.Background(), name)
	}
	if err != nil {
		return err
	}
	screen, err := backend.Capture(context.Background(), pane.ID)
	if err != nil {
		return err
	}
	text := renderScreen(screen, lineCount)
	if text == "" {
		return nil
	}
	_, err = fmt.Fprintln(cmd.stdout, text)
	return err
}

func renderScreen(screen substrate.Screen, lineCount int) string {
	rows := make([]string, len(screen.Rows))
	for rowNumber, row := range screen.Rows {
		var line strings.Builder
		for _, cell := range row {
			line.WriteString(cell.Text)
		}
		rows[rowNumber] = strings.TrimRight(line.String(), " ")
	}
	for len(rows) != 0 && rows[len(rows)-1] == "" {
		rows = rows[:len(rows)-1]
	}
	if lineCount > 0 && len(rows) > lineCount {
		rows = rows[len(rows)-lineCount:]
	}
	return strings.Join(rows, "\n")
}
