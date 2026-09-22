package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/store"
)

func (cmd command) statusline(arguments []string) (result error) {
	if len(arguments) == 1 && arguments[0] == "--install" {
		home, err := cmd.userHomeDir()
		if err != nil {
			return err
		}
		executable, err := os.Executable()
		if err != nil {
			return err
		}
		changed, err := harness.InstallStatusline(filepath.Join(home, ".claude", "settings.json"), executable)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(cmd.stdout, "status-line settings updated: %t\n", changed)
		return err
	}
	if err := noArguments(arguments, "statusline"); err != nil {
		return err
	}
	defer func() {
		id := core.HitchID(cmd.environment("GANGLINE_HITCH_ID"))
		if result == nil || id == "" {
			return
		}
		run, err := cmd.runtime()
		if err != nil {
			result = errors.Join(result, err)
			return
		}
		invocation, err := randomID("statusline")
		if err != nil {
			result = errors.Join(result, err)
			return
		}
		result = errors.Join(result, run.recordNativeHook(core.NativeHook{At: time.Now(), ID: invocation, HitchID: id, NativeEvent: "statusline", Status: "failed", Reason: result.Error()}))
	}()
	data, err := io.ReadAll(io.LimitReader(cmd.stdin, maximumMessageBytes+1))
	if err != nil {
		return err
	}
	if len(data) > maximumMessageBytes {
		return refuseError("status-line payload exceeds maximum size")
	}
	session, readings, err := harness.ReadStatusline(data)
	if err != nil {
		return err
	}
	if readings[0].Status == "observed" {
		at, err := harness.StatuslineMeasurementTime(data, readings[0])
		if err != nil {
			return err
		}
		readings[0].At = at
	}
	r := coreReading(readings[0])
	if id := core.HitchID(cmd.environment("GANGLINE_HITCH_ID")); id != "" {
		run, err := cmd.runtime()
		if err != nil {
			return err
		}
		if err := run.recordStatusline(id, session, readings); err != nil {
			return err
		}
		latest, err := run.latestReadings(id)
		if err != nil {
			return err
		}
		r = latest.Context
	}
	if r.Status != "observed" {
		_, err = fmt.Fprintln(cmd.stdout, "context unknown: "+r.Reason)
		return err
	}
	_, err = fmt.Fprintf(cmd.stdout, "context %d/%d (%.0f%%)\n", *r.Used, *r.Limit, *r.Percent)
	return err
}

func (run *runtime) recordStatusline(id core.HitchID, session string, readings []harness.Reading) (result error) {
	var target core.Hitch
	defer func() {
		if target.ID != "" {
			result = errors.Join(result, run.publishContext(target))
		}
	}()
	locked, err := run.lock()
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, locked.Close()) }()
	state, _, err := locked.Load(run.initial())
	if err != nil {
		return err
	}
	hitch, ok := state.Hitches[id]
	if !ok {
		return refuseError("status-line hitch %q is not registered", id)
	}
	if hitch.Status == core.HitchDropped || hitch.Status == core.HitchDropping || hitch.Status == core.HitchFailed {
		return nil
	}
	target = hitch
	entries, err := locked.Log()
	if err != nil {
		return err
	}
	h := historyFor(entries, id)
	if h.SessionID != "" && h.SessionID != session {
		return fmt.Errorf("status-line session %q does not match hitch session %q", session, h.SessionID)
	}
	observation := core.Observation{At: time.Now(), HitchID: id, Collar: hitch.Collar, SessionID: session}
	for _, r := range readings {
		// Status-line JSON has no native measurement timestamp. After compaction,
		// even a changed value may be a delayed pre-compaction callback.
		if r.Kind == "context" && h.InvalidContext != nil && (r.At == nil || !r.At.After(h.InvalidAfter)) {
			r = harness.UnknownReading("context", "status-line", "status-line has no measurement timestamp; post-compaction context freshness is unknown")
		}
		observation.Readings = append(observation.Readings, coreReading(r))
	}
	if err := locked.Append(observation); err != nil {
		return err
	}
	entries = append(entries, store.LogEntry{Event: observation})
	return errors.Join(run.writeLatest(id, historyFor(entries, id)))
}
