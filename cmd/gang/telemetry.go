package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/store"
)

func coreReading(r harness.Reading) core.Reading {
	out := core.Reading{Kind: r.Kind, Source: r.Source, NativeEvent: r.NativeEvent, At: r.At, Status: r.Status, Reason: r.Reason, TurnID: r.TurnID, Model: r.Model, Used: r.Used, Limit: r.Limit, Percent: r.Percent}
	out.SessionID, out.SourceOffset = r.SessionID, r.SourceOffset
	if u := r.Usage; u != nil {
		out.Usage = &core.TokenUsage{Semantics: u.Semantics, Fields: u.Fields, CacheInclusion: u.CacheInclusion, Input: u.Input, Output: u.Output, CachedInput: u.CachedInput}
	}
	for _, w := range r.Limits {
		out.Limits = append(out.Limits, core.LimitWindow{Label: w.Label, UsedPercent: w.UsedPercent, ResetAt: w.ResetAt, WindowMinutes: w.WindowMinutes})
	}
	return out
}
func acceptReadings(n *core.NativeState, readings []core.Reading) {
	for _, r := range readings {
		switch r.Kind {
		case "compaction-checkpoint", "compaction-finished":
			if r.Kind == "compaction-finished" && r.At != nil && r.At.After(n.ConfirmedCompactedAt) {
				n.ConfirmedCompactedAt = *r.At
			}
			if r.At != nil && r.At.After(n.CompactedAt) {
				n.CompactedAt = *r.At
				if n.Context.At == nil || !n.Context.At.After(*r.At) {
					n.Context = core.Reading{Kind: "context", Source: r.Source, Status: "unknown", Reason: "compaction completed; waiting for a newer native measurement"}
				}
			}
		case "model":
			n.Model = r.Model
		case "context":
			if n.Context.At != nil && (r.At == nil || r.At.Before(*n.Context.At)) {
				continue
			}
			if !n.CompactedAt.IsZero() && (r.At == nil || !r.At.After(n.CompactedAt)) {
				continue
			}
			if r.Model == "" {
				r.Model = n.Model
			}
			n.Context = r
		case "provider-limits":
			if n.Limits.At != nil && (r.At == nil || r.At.Before(*n.Limits.At)) {
				continue
			}
			n.Limits = r
		case "error":
			if r.At != nil && r.At.After(n.LastErrorAt) {
				n.LastErrorAt = *r.At
				n.LastError = r.Reason
			}
		}
	}
}

// acceptTurnFailures records a native turn failure that only the session log
// reports. The failure describes the agent only when the submit witness names
// its turn: an older turn's failure is history, and a later submit clears it.
func acceptTurnFailures(l *store.LockedAgent, a *core.Agent, readings []core.Reading) error {
	for _, r := range readings {
		if r.Kind != "turn-failed" || r.TurnID == "" || r.At == nil {
			continue
		}
		witness, err := l.Paths.ReadWitness()
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		if witness.TurnID != r.TurnID {
			continue
		}
		a.Native.TurnFailure, a.Native.FailedTurn = boundedFailureReason(r.Reason), r.TurnID
		// Gang records its own delivery after the submit hook ran, so the
		// failed turn closes no earlier than that submission.
		finished := *r.At
		if a.Native.SubmittedAt.After(finished) {
			finished = a.Native.SubmittedAt
		}
		if finished.After(a.Native.FinishedAt) {
			a.Native.FinishedAt = finished
		}
	}
	return nil
}
func (run *runtime) refreshNative(l *store.LockedAgent, a *core.Agent, c harness.Collar) error {
	if c.Primitives.Telemetry == nil || !harness.TelemetryUsesTranscript(*c.Primitives.Telemetry) || a.Native.Transcript == "" {
		return nil
	}
	f, err := harness.OpenTranscript(a.Native.Transcript)
	if err != nil {
		return err
	}
	w := a.Native.Context
	comparison := harness.CompareTranscriptContext(f, a.Native.SessionID, a.Native.Offset, harness.Reading{
		Kind: w.Kind, Source: w.Source, NativeEvent: w.NativeEvent, Status: w.Status,
		SessionID: w.SessionID, SourceOffset: w.SourceOffset, At: w.At, Used: w.Used, Limit: w.Limit,
	})
	parsed, err := harness.ReadTranscript(*c.Primitives.Telemetry, f, a.Native.SessionID, a.Native.Offset, a.CreatedAt)
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if parsed.Offset == a.Native.Offset && comparison != "mismatch" {
		return nil
	}
	var readings []core.Reading
	if comparison == "mismatch" {
		readings = append(readings, core.Reading{Kind: "observation-gap", Source: "session-log",
			SessionID: a.Native.SessionID, SourceOffset: w.SourceOffset,
			NativeEvent: "source-context-mismatch", Status: "unknown"})
	}
	for _, r := range parsed.Readings {
		readings = append(readings, coreReading(r))
	}
	// Do not acknowledge source bytes before their observations are retained.
	// A state-write failure can replay the same source offsets on retry, but
	// an audit failure must not permanently skip those observations.
	if len(readings) > 0 {
		// Context identity is a saved-state witness. Its paired usage already
		// carries the identity in history; avoid growing every context audit.
		auditReadings := append([]core.Reading(nil), readings...)
		for i := range auditReadings {
			if auditReadings[i].Kind == "context" && auditReadings[i].Source == "session-log" {
				auditReadings[i].SessionID, auditReadings[i].SourceOffset = "", 0
			}
		}
		if err := run.record(*a, core.Event{Type: "observation", Readings: auditReadings}); err != nil {
			return err
		}
	}
	next := *a
	// Native completion mutates the pending compaction. Keep that update
	// private until the cursor and completion have been saved together.
	if a.Compaction != nil {
		pending := *a.Compaction
		next.Compaction = &pending
	}
	if comparison == "mismatch" {
		next.Native.Context = core.Reading{Kind: "context", Source: "session-log", Status: "unknown",
			Reason: "saved context source record changed; waiting for a newer native measurement"}
	}
	if err := run.acceptContextReadings(&next, c, readings); err != nil {
		return err
	}
	if err := acceptTurnFailures(l, &next, readings); err != nil {
		return err
	}
	next.Native.Offset = parsed.Offset
	if err := l.Save(next); err != nil {
		return err
	}
	*a = next
	if err := run.publishContextNotes(l, a); err != nil {
		return err
	}
	if len(readings) > 0 {
		return run.publishContext(*a)
	}
	return nil
}
func (run *runtime) latest(a core.Agent) (core.Agent, error) {
	l, updated, err := run.acquire(a.ID, false)
	if err == store.ErrLocked {
		return a, nil
	}
	if err != nil {
		return a, err
	}
	defer run.unlock(l)
	if err := run.checkDeadlines(l, &updated); err != nil {
		return a, err
	}
	c, err := loadCollar(updated.Collar, run.settings)
	if err != nil {
		return a, err
	}
	if err := run.refreshNative(l, &updated, c); err != nil {
		return a, err
	}
	if err := run.release(l); err != nil {
		return a, err
	}
	return updated, nil
}
func (run *runtime) publishContext(a core.Agent) error {
	if run.cmd.inputBackend != nil {
		return nil
	}
	b, err := run.cmd.tmux(run.settings)
	if err != nil {
		return err
	}
	return b.PublishContext(context.Background(), string(a.ID), contextWidgetText(string(a.Name), a.Native.Context))
}
func contextWidgetText(name string, r core.Reading) string {
	if r.Status != "observed" || r.Percent == nil {
		return name + " ?"
	}
	if r.Used != nil && r.Limit != nil {
		return name + " " + contextUsageText(*r.Used, *r.Limit, *r.Percent)
	}
	return fmt.Sprintf("%s %.0f%%", name, *r.Percent)
}
