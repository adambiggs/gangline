package main

import (
	"encoding/json"
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
	return cmd.observeWedges(run, state)
}
func (cmd command) hook(arguments []string) error {
	if err := noArguments(arguments, "hook"); err != nil {
		return err
	}
	id := core.HitchID(cmd.environment("GANGLINE_HITCH_ID"))
	if id == "" {
		return refuseError("hook has no GANGLINE_HITCH_ID")
	}
	run, err := cmd.runtime()
	if err != nil {
		return err
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
		_, err = run.drive(core.TurnBoundaryReached{At: now, HitchID: id})
	case harness.TurnCompactionFinished:
		if hitch.PendingCompactID != "" {
			compact := state.Compactions[hitch.PendingCompactID]
			_, err = run.drive(core.CompactionCompleted{At: now, CompactionID: hitch.PendingCompactID})
			if err == nil {
				id, idErr := randomID("resume")
				if idErr != nil {
					return idErr
				}
				_, err = run.drive(core.SendRequested{
					At: now, Deadline: now.Add(deliveryTimeout),
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
