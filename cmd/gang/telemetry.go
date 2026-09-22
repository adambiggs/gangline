package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/store"
)

type observationHistory struct {
	Model      string
	SessionID  string
	Transcript string
	Offset     int64
	Since      time.Time
	Context    core.Reading
	Limits     core.Reading
	// After compaction, an unversioned status-line payload cannot prove freshness.
	InvalidContext *core.Reading
	InvalidAfter   time.Time
}

func historyFor(entries []store.LogEntry, id core.HitchID) observationHistory {
	h := observationHistory{Context: coreReading(harness.UnknownReading("context", "native-hook", "no native context reading recorded; expected collar status-line payload or session log")), Limits: coreReading(harness.UnknownReading("provider-limits", "native-hook", "no native provider-limit reading recorded"))}
	for _, entry := range entries {
		if e, ok := entry.Event.(core.HitchRequested); ok && e.Hitch.ID == id {
			h.Since = e.At
		}
		if e, ok := entry.Event.(core.AdoptRequested); ok && e.Hitch.ID == id {
			h.Since = e.At
		}
		if e, ok := entry.Event.(core.Observation); ok {
			if e.HitchID != id {
				continue
			}
			if e.SessionID != "" {
				h.SessionID = e.SessionID
			}
			if e.Transcript != "" {
				h.Transcript = e.Transcript
				h.Offset = e.Offset
			}
			for _, r := range e.Readings {
				switch r.Kind {
				case "model":
					h.Model = r.Model
				case "context":
					if r.Model == "" {
						r.Model = h.Model
					}
					if h.Context.At != nil && (r.At == nil || r.At.Before(*h.Context.At)) {
						continue
					}
					if h.InvalidContext != nil && (r.At == nil || !r.At.After(h.InvalidAfter)) {
						continue
					}
					h.Context = r
					if r.Status == "observed" && r.At != nil {
						h.InvalidContext = nil
					}
				case "provider-limits":
					h.Limits = r
				case "compaction-finished", "compaction-checkpoint":
					after := e.At
					if r.At != nil {
						after = *r.At
					}
					if h.Context.At != nil && h.Context.At.After(after) {
						continue
					}
					copy := h.Context
					h.InvalidContext = &copy
					h.InvalidAfter = after
					h.Context = coreReading(harness.UnknownReading("context", r.Source, "compaction completed; waiting for a provably newer native context reading"))
				}
			}
		}
	}
	return h
}

func coreReading(r harness.Reading) core.Reading {
	out := core.Reading{Kind: r.Kind, Source: r.Source, NativeEvent: r.NativeEvent, At: r.At, Status: r.Status, Reason: r.Reason, Model: r.Model, Used: r.Used, Limit: r.Limit, Percent: r.Percent}
	for _, w := range r.Limits {
		out.Limits = append(out.Limits, core.LimitWindow{Label: w.Label, UsedPercent: w.UsedPercent, ResetAt: w.ResetAt})
	}
	return out
}

func (run *runtime) observeHook(hitch core.Hitch, collar harness.Collar, event harness.HookEvent) (result error) {
	defer func() {
		if event.NativeEvent != "" {
			result = errors.Join(result, run.publishContext(hitch))
		}
	}()
	locked, err := run.lock()
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, locked.Close()) }()
	entries, err := locked.Log()
	if err != nil {
		return err
	}
	history := historyFor(entries, hitch.ID)
	observation := core.Observation{Readings: []core.Reading{}, At: time.Now(), HitchID: hitch.ID, Collar: hitch.Collar, SessionID: history.SessionID, Transcript: history.Transcript, Offset: history.Offset}
	session := event.Payload["session_id"]
	if session != "" {
		if history.SessionID != "" && session != history.SessionID {
			return fmt.Errorf("native hook session changed from %q to %q for hitch %s", history.SessionID, session, hitch.ID)
		}
		observation.SessionID = session
	}
	var observationErr error
	path := event.Payload["transcript_path"]
	if path == "" {
		path = history.Transcript
	}
	if collar.Primitives.Telemetry != nil && harness.TelemetryUsesTranscript(*collar.Primitives.Telemetry) && path != "" {
		if history.Transcript != "" && path != history.Transcript {
			observationErr = fmt.Errorf("native transcript path changed for hitch %s", hitch.ID)
		} else {
			f, err := os.Open(path)
			if err != nil {
				observationErr = err
			} else {
				parsed, err := harness.ReadTranscript(*collar.Primitives.Telemetry, f, observation.SessionID, history.Offset, history.Since)
				observationErr = errors.Join(err, f.Close())
				if observationErr == nil {
					observation.Transcript = path
					observation.Offset = parsed.Offset
					for _, r := range parsed.Readings {
						observation.Readings = append(observation.Readings, coreReading(r))
					}
				}
			}
		}
	}
	if observationErr != nil {
		observation.Readings = append(observation.Readings, core.Reading{Kind: "error", Source: "session-log", Status: "error", Reason: observationErr.Error()})
	}
	// Native hook facts retain their own source. Session-log records are separate
	// witnesses and must not be summed with hooks as independent turns.
	native := core.Reading{Kind: event.Kind, Source: "native-hook", NativeEvent: event.NativeEvent, Status: "observed", Reason: event.Payload["error"]}
	if event.Kind == "compaction-finished" {
		for _, reading := range observation.Readings {
			if reading.Kind == "compaction-finished" || reading.Kind == "compaction-checkpoint" {
				native.At = reading.At
			}
		}
	}
	if event.NativeEvent != "" {
		if event.Kind == "turn-failed" {
			native.Kind = "error"
			observation.Readings = append(observation.Readings, native)
			native.Kind = "turn-finished"
		}
		observation.Readings = append(observation.Readings, native)
	}
	if event.Kind == "turn-started" || event.Kind == "turn-finished" || event.Kind == "compaction-started" || event.Kind == "compaction-finished" || event.Kind == "turn-failed" {
		projected := append(append([]store.LogEntry(nil), entries...), store.LogEntry{Event: observation})
		latest := historyFor(projected, hitch.ID)
		context := latest.Context
		if observationErr != nil {
			context = coreReading(harness.UnknownReading("context", "session-log", observationErr.Error()))
		}
		// A boundary snapshot keeps the actual reading timestamp/source, if any.
		observation.Readings = append(observation.Readings, context, latest.Limits)
	}
	if len(observation.Readings) == 0 && observation.Offset == history.Offset {
		return observationErr
	}
	if err := locked.Append(observation); err != nil {
		return errors.Join(observationErr, err)
	}
	entries = append(entries, store.LogEntry{Event: observation})
	return errors.Join(observationErr, run.writeLatest(hitch.ID, historyFor(entries, hitch.ID)))
}

func (run *runtime) latestPath(id core.HitchID) (string, error) {
	paths, err := run.paths().Team(run.settings.Session)
	if err != nil {
		return "", err
	}
	if id == "" || filepath.Base(string(id)) != string(id) {
		return "", fmt.Errorf("invalid hitch identity %q", id)
	}
	return filepath.Join(paths.Directory, "readings", string(id)+".json"), nil
}

func atomicJSON(path string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".reading-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(append(data, '\n')); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}

func (run *runtime) writeLatest(id core.HitchID, h observationHistory) error {
	path, err := run.latestPath(id)
	if err != nil {
		return err
	}
	return atomicJSON(path, struct {
		HitchID   core.HitchID `json:"hitch_id"`
		SessionID string       `json:"session_id"`
		Context   core.Reading `json:"context"`
		Limits    core.Reading `json:"provider_limits"`
	}{id, h.SessionID, h.Context, h.Limits})
}

func (run *runtime) latestReadings(id core.HitchID) (observationHistory, error) {
	locked, err := run.lock()
	if err != nil {
		return observationHistory{}, err
	}
	defer locked.Close()
	entries, err := locked.Log()
	if err != nil {
		return observationHistory{}, err
	}
	h := historyFor(entries, id)
	return h, run.writeLatest(id, h)
}

func (run *runtime) publishContext(hitch core.Hitch) error {
	paths, err := run.paths().Team(run.settings.Session)
	if err != nil {
		return err
	}
	marker, err := os.ReadFile(filepath.Join(paths.Directory, "context-widget.json"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var target string
	if err := json.Unmarshal(marker, &target); err != nil {
		return err
	}
	if target != string(hitch.ID) {
		return nil
	}
	path, err := run.latestPath(hitch.ID)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var latest struct {
		Context core.Reading `json:"context"`
	}
	if err := json.Unmarshal(data, &latest); err != nil {
		return err
	}
	backend, err := run.cmd.tmux(run.settings)
	if err != nil {
		return err
	}
	return backend.PublishContext(context.Background(), string(hitch.ID), contextWidgetText(string(hitch.Name), latest.Context))
}
