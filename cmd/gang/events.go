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

func (cmd command) hook(arguments []string) (result error) {
	if err := noArguments(arguments, "hook"); err != nil {
		return err
	}
	// Drain the native input before waiting on any team transaction.
	payload, readErr := io.ReadAll(io.LimitReader(cmd.stdin, maximumMessageBytes+1))
	run, err := cmd.runtime()
	if err != nil {
		return err
	}
	invocation, err := randomID("hook")
	if err != nil {
		return err
	}
	id := core.HitchID(cmd.environment("GANGLINE_HITCH_ID"))
	var native struct {
		Name  string `json:"hook_event_name"`
		Event string `json:"event"`
	}
	_ = json.Unmarshal(payload, &native)
	if native.Name == "" {
		native.Name = native.Event
	}
	observation := core.NativeHook{At: time.Now(), ID: invocation, HitchID: id, NativeEvent: native.Name, Status: "received"}
	defer func() {
		observation.At = time.Now()
		if result != nil {
			observation.Status, observation.Reason = "failed", result.Error()
		} else if observation.Status != "ignored" {
			observation.Status = "completed"
		}
		if err := run.recordNativeHook(observation); err != nil {
			result = errors.Join(result, fmt.Errorf("record native hook outcome: %w", err))
		}
	}()
	if err := run.recordNativeHook(observation); err != nil {
		return err
	}
	if readErr != nil {
		return readErr
	}
	if len(payload) > maximumMessageBytes {
		return refuseError("native hook payload exceeds maximum size")
	}
	if id == "" {
		return refuseError("hook has no GANGLINE_HITCH_ID")
	}
	state, err := run.load()
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

	boundary, hookEvent, err := harness.DetectTurnBoundary(collar, payload)
	if err != nil {
		return err
	}
	if hitch.Status == core.HitchDropping || hitch.Status == core.HitchDropped || hitch.Status == core.HitchFailed {
		observation.Status, observation.Reason = "ignored", "recipient is no longer active"
		return nil
	}
	// Activity does not change lifecycle state. Avoid scanning unrelated panes on
	// every tool hook; boundary and permission hooks retain direct observation.
	if hookEvent.Kind == "activity" {
		return nil
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
		if openErr != nil && !errors.Is(openErr, syscall.ENOENT) && !errors.Is(openErr, syscall.ENXIO) {
			return fmt.Errorf("open native submission witness: %w", openErr)
		}
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
					At: now,
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

// Recording the hook itself does not depend on loading or reducing team state:
// a load/decode failure must still leave its diagnostic in the event log.
func (run *runtime) recordNativeHook(event core.NativeHook) error {
	locked, err := run.lock()
	if err != nil {
		return err
	}
	appendErr := locked.Append(event)
	return errors.Join(appendErr, locked.Close())
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
