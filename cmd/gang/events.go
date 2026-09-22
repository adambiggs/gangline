package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"syscall"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/store"
)

func (cmd command) tick(arguments []string) error {
	if err := noArguments(arguments, "tick"); err != nil {
		return err
	}
	run, err := cmd.runtime()
	if err != nil {
		return err
	}
	state, err := run.recover()
	if err != nil {
		return err
	}
	return run.observeWedges(state)
}

func (cmd command) hook(arguments []string) error {
	if err := noArguments(arguments, "hook"); err != nil {
		return err
	}
	id := core.HitchID(cmd.environment("GANGLINE_HITCH_ID"))
	if id == "" {
		return refuseError("hook has no GANGLINE_HITCH_ID")
	}
	run, state, err := cmd.loaded()
	if err != nil {
		return err
	}
	hitch, ok := state.Hitches[id]
	if !ok {
		return refuseError("hook hitch %q is not registered", id)
	}
	collar, err := loadCollar(hitch.Collar, run.settings)
	if err != nil {
		return err
	}
	payload, err := io.ReadAll(io.LimitReader(cmd.stdin, maximumMessageBytes+1))
	if err != nil {
		return err
	}
	boundary, hookEvent, err := harness.DetectTurnBoundary(collar, payload)
	if err != nil {
		return err
	}
	state, err = run.refreshBlocked(state)
	if err != nil {
		return err
	}
	hitch = state.Hitches[id]
	if hookEvent.Kind == "permission-requested" {
		if hitch.Activity != core.ActivityBlocked {
			_, err = run.drive(core.BlockedDetected{
				At: time.Now(), HitchID: id, Evidence: "native permission request reported by collar hook",
			})
		}
		return err
	}
	if boundary == harness.TurnFinished && hitch.Activity == core.ActivityBlocked {
		state, err = run.drive(core.BlockedCleared{At: time.Now(), HitchID: id})
		if err != nil {
			return err
		}
		hitch = state.Hitches[id]
	}
	if boundary == harness.TurnStarted && hookEvent.Payload["prompt"] != "" {
		witness := run.deliveryWitnessPath(id)
		file, openErr := os.OpenFile(witness, os.O_WRONLY|syscall.O_NONBLOCK, 0)
		if openErr == nil {
			_, writeErr := io.WriteString(file, hookEvent.Payload["prompt"])
			closeErr := file.Close()
			if writeErr != nil {
				return writeErr
			}
			if closeErr != nil {
				return closeErr
			}
		}
	}
	now := time.Now()
	switch boundary {
	case harness.TurnStarted:
		if hitch.Activity == core.ActivityIdle {
			_, err = run.drive(core.TurnStarted{At: now, HitchID: id})
		}
	case harness.TurnFinished:
		state, err = run.drive(core.TurnBoundaryReached{At: now, HitchID: id})
	case harness.TurnCompactionFinished:
		if hitch.PendingCompactID != "" {
			compact := state.Compactions[hitch.PendingCompactID]
			state, err = run.drive(core.CompactionCompleted{At: now, CompactionID: hitch.PendingCompactID})
			if err == nil {
				id, idErr := randomID("resume")
				if idErr != nil {
					return idErr
				}
				state, err = run.drive(core.SendRequested{
					At: now, Deadline: now.Add(run.deliveryBudget()),
					Envelope: core.Envelope{
						ID: core.EnvelopeID(id), From: core.Sender{Kind: core.SenderSelfDeclared, Name: "compact"},
						To: hitch.Name, Message: compact.Resume, CreatedAt: now,
					},
				})
			}
		}
	case harness.TurnCompactionStarted:
		// The request event already records the durable start intent.
	default:
		// Activity and permission hooks are valid evidence but not boundaries.
	}
	if err == nil && (boundary == harness.TurnFinished || boundary == harness.TurnCompactionFinished) {
		return run.finishBoundaryDeliveries(state, hitch.ID)
	}
	return err
}

func (cmd command) log(arguments []string) error {
	if len(arguments) != 0 {
		return usageError("log filters are not available in the v1 event log; use gang log")
	}
	run, err := cmd.runtime()
	if err != nil {
		return err
	}
	paths, err := run.paths().Team(run.settings.Session)
	if err != nil {
		return err
	}
	file, err := os.Open(paths.Events)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(cmd.stdout, file)
	return err
}

func (cmd command) replay(arguments []string) error {
	if len(arguments) > 1 {
		return usageError("replay: expected at most one event log")
	}
	reader := cmd.stdin
	if len(arguments) == 1 {
		file, err := os.Open(arguments[0])
		if err != nil {
			return err
		}
		defer file.Close()
		reader = file
	}
	entries, err := store.ReadLog(reader)
	if err != nil {
		return err
	}
	state := store.Replay(core.NewState(core.Team{ID: "replay", Name: "replay"}), entries)
	encoder := json.NewEncoder(cmd.stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(state)
}

func (cmd command) wait(arguments []string) error {
	options, err := parseWait(arguments)
	if err != nil {
		return err
	}
	run, err := cmd.runtime()
	if err != nil {
		return err
	}

	deadline := time.Now().Add(options.Timeout)
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()
	paths, err := run.paths().Team(run.settings.Session)
	if err != nil {
		return err
	}
	for {
		state, size, complete, err := store.ObserveLog(paths.Events, run.initial())
		if err != nil {
			return err
		}
		hitch, ok := activeByName(state, options.Name)
		if !ok {
			return refuseError("agent %q is not active", options.Name)
		}
		if complete && hitch.Activity == core.ActivityIdle {
			return nil
		}
		if !time.Now().Before(deadline) {
			if !complete {
				return fmt.Errorf("event log has an incomplete append at the wait deadline")
			}
			return run.waitTimedOut(hitch, deadline)
		}
		appendWait, watchErr := cmd.appendWait(paths.Events, size)
		if watchErr != nil {
			return watchErr
		}
		waitErr := appendWait.Wait(ctx)
		closeErr := appendWait.Close()
		if waitErr != nil {
			if errors.Is(waitErr, context.DeadlineExceeded) {
				return run.waitTimedOut(hitch, deadline)
			}
			return waitErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
}

type appendWait interface {
	Wait(context.Context) error
	Close() error
}

func (cmd command) appendWait(path string, after int64) (appendWait, error) {
	if cmd.newAppendWait != nil {
		return cmd.newAppendWait(path, after)
	}
	return store.NewAppendWait(path, after)
}

func (run *runtime) waitTimedOut(hitch core.Hitch, deadline time.Time) error {
	now := time.Now()
	_, err := run.drive(core.OperationTimedOut{
		At: now, Operation: core.TimeoutWait, ID: string(hitch.ID), Deadline: deadline,
		Evidence: "wait deadline elapsed before an idle boundary",
	})
	if err != nil {
		return err
	}
	return refuseError("agent %q did not reach an idle boundary before the wait deadline", hitch.Name)
}
