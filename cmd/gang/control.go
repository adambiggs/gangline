package main

import (
	"context"
	"strings"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/substrate"
)

func (cmd command) interrupt(arguments []string) error {
	name := ""
	if len(arguments) != 0 && !strings.HasPrefix(arguments[0], "-") {
		name, arguments = arguments[0], arguments[1:]
	}
	reason := ""
	flags := quietFlagSet("interrupt")
	flags.StringVar(&reason, "m", "", "reason")
	if err := flags.Parse(arguments); err != nil || flags.NArg() != 0 {
		return usageError("interrupt: invalid arguments")
	}
	run, err := cmd.runtime()
	if err != nil {
		return err
	}
	state, err := run.load()
	if err != nil {
		return err
	}
	if name == "" {
		name = string(state.Team.Name)
		if pane := cmd.environment("TMUX_PANE"); pane != "" {
			for _, hitch := range state.Hitches {
				if hitch.Pane == pane {
					name = string(hitch.Name)
					break
				}
			}
		}
	}
	hitch, ok := activeByName(state, name)
	if !ok {
		return refuseError("agent %q is not active", name)
	}
	if hitch.Activity != core.ActivityBusy && hitch.Activity != core.ActivityWedged {
		return refuseError("agent %q is not in an interruptible turn", name)
	}
	now := time.Now()
	_, err = run.drive(core.InterruptRequested{At: now, HitchID: hitch.ID, Reason: reason, Deadline: now.Add(operationTimeout)})
	return err
}
func (cmd command) compact(arguments []string) error {
	options, err := parseCompact(arguments)
	if err != nil {
		return err
	}
	run, err := cmd.runtime()
	if err != nil {
		return err
	}
	state, err := run.load()
	if err != nil {
		return err
	}
	name := options.Name
	if name == "" {
		pane := cmd.environment("TMUX_PANE")
		for _, hitch := range state.Hitches {
			if hitch.Pane == pane {
				name = string(hitch.Name)
				break
			}
		}
	}
	hitch, ok := activeByName(state, name)
	if !ok {
		return refuseError("agent %q is not active", name)
	}
	if options.Recover {
		collar, err := loadCollar(hitch.Collar, run.settings)
		if err != nil {
			return err
		}
		backend, err := cmd.tmux(run.settings)
		if err != nil {
			return err
		}
		for _, action := range collar.Actions.CompactRecover {
			if err := sendHarnessKeys(context.Background(), backend, substrate.PaneID(hitch.Pane), collar, action.Input()); err != nil {
				return err
			}
		}
		if hitch.PendingCompactID != "" {
			_, err = run.drive(core.CompactionFailedEvent{At: time.Now(), CompactionID: hitch.PendingCompactID, Reason: "operator requested native compaction recovery"})
		}
		return err
	}
	resume := options.Resume
	if resume == "" {
		resume = "Your context was just compacted. Re-read your brief and durable state, then resume your lane or report it complete."
	}
	id, err := randomID("compact")
	if err != nil {
		return err
	}
	now := time.Now()
	_, err = run.drive(core.CompactionRequested{At: now, Compaction: core.Compaction{ID: core.CompactionID(id), HitchID: hitch.ID, Resume: core.Message{Text: resume}, Deadline: now.Add(operationTimeout)}})
	return err
}
