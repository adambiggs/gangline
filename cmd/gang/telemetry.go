package main

import (
	"context"
	"fmt"
	"os"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/store"
)

func coreReading(r harness.Reading) core.Reading {
	out := core.Reading{Kind: r.Kind, Source: r.Source, NativeEvent: r.NativeEvent, At: r.At, Status: r.Status, Reason: r.Reason, Model: r.Model, Used: r.Used, Limit: r.Limit, Percent: r.Percent}
	for _, w := range r.Limits {
		out.Limits = append(out.Limits, core.LimitWindow{Label: w.Label, UsedPercent: w.UsedPercent, ResetAt: w.ResetAt})
	}
	return out
}
func acceptReadings(n *core.NativeState, readings []core.Reading) {
	for _, r := range readings {
		switch r.Kind {
		case "compaction-checkpoint", "compaction-finished":
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
			n.Limits = r
		}
	}
}
func (run *runtime) refreshNative(l *store.LockedAgent, a *core.Agent, c harness.Collar) error {
	if c.Primitives.Telemetry == nil || !harness.TelemetryUsesTranscript(*c.Primitives.Telemetry) || a.Native.Transcript == "" {
		return nil
	}
	f, err := os.Open(a.Native.Transcript)
	if err != nil {
		return err
	}
	parsed, err := harness.ReadTranscript(*c.Primitives.Telemetry, f, a.Native.SessionID, a.Native.Offset, a.CreatedAt)
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if parsed.Offset == a.Native.Offset {
		return nil
	}
	var readings []core.Reading
	for _, r := range parsed.Readings {
		readings = append(readings, coreReading(r))
	}
	acceptReadings(&a.Native, readings)
	a.Native.Offset = parsed.Offset
	if err := l.Save(*a); err != nil {
		return err
	}
	if len(readings) > 0 {
		if err := run.record(*a, core.Event{Type: "observation", Readings: readings}); err != nil {
			return err
		}
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
	defer l.Close()
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
		return name + " context ?"
	}
	return fmt.Sprintf("%s context %.0f%%", name, *r.Percent)
}
