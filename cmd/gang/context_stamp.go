package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/store"
	"github.com/adambiggs/gangline/substrate"
)

// Match context's native verification rules without acquiring this lock again.
func deliveryReadings(a core.Agent, c harness.Collar, b harnessInput) (core.Reading, []core.Reading) {
	r := a.Native.Context
	var updates []core.Reading
	if c.Primitives.Telemetry != nil {
		if harness.TelemetryUsesTranscript(*c.Primitives.Telemetry) && a.Native.SessionID != "" && a.Native.Transcript != "" {
			path, err := harness.ResolveTranscript(*c.Primitives.Telemetry, a.Native.Transcript, a.Native.SessionID)
			if err != nil {
				return core.Reading{}, nil
			}
			file, err := harness.OpenTranscript(path)
			if err != nil {
				return core.Reading{}, nil
			}
			comparison := harness.CompareTranscriptContext(file, a.Native.SessionID, a.Native.Offset, harness.Reading{
				Kind: r.Kind, Source: r.Source, NativeEvent: r.NativeEvent, Status: r.Status,
				SessionID: r.SessionID, SourceOffset: r.SourceOffset, At: r.At, Used: r.Used, Limit: r.Limit,
			})
			parsed, err := harness.ReadTranscript(*c.Primitives.Telemetry, file, a.Native.SessionID, a.Native.Offset, a.CreatedAt)
			closeErr := file.Close()
			if err != nil || closeErr != nil {
				return core.Reading{}, nil
			}
			native := a.Native
			if comparison != "match" {
				native.Context = core.Reading{}
			}
			for _, reading := range parsed.Readings {
				update := coreReading(reading)
				updates = append(updates, update)
				acceptReadings(&native, []core.Reading{update})
			}
			r = native.Context
		}
		return r, updates
	}
	screen, err := b.Capture(context.Background(), substrate.PaneID(a.Pane))
	if err != nil {
		return core.Reading{}, nil
	}
	reading, err := harness.ReadContext(c.Primitives.Context, screen)
	if err != nil {
		return core.Reading{}, nil
	}
	percent := reading.Percent * 100
	model := ""
	if c.Models.Selected != nil {
		model, _ = harness.ReadSelectedModel(*c.Models.Selected, screen)
	}
	return core.Reading{Kind: "context", Status: "observed", Model: model, Used: &reading.Used, Limit: &reading.Limit, Percent: &percent}, nil
}

func contextStampText(r core.Reading) string {
	if r.Status != "observed" || r.Used == nil || r.Limit == nil || r.Percent == nil {
		return "ctx=unknown"
	}
	return fmt.Sprintf("ctx=%d/%d", *r.Used, *r.Limit)
}

func (run *runtime) stampEnvelope(l *store.LockedAgent, a *core.Agent, e core.Envelope, b harnessInput, c harness.Collar) (core.Envelope, error) {
	stored, err := l.Paths.ReadEnvelope("new", e.ID)
	if err == nil && stored.ContextStamp != nil {
		e = stored
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return e, err
	}
	if e.ContextStamp != nil {
		return e, nil
	}
	r, updates := deliveryReadings(*a, c, b)
	if err := run.acceptContextReadings(a, c, updates); err != nil {
		return e, err
	}
	if c.Primitives.Telemetry == nil && r.Status == "observed" {
		at := run.cmd.now()
		r.At, r.Source = &at, "screen"
	}
	if r.Kind == "context" {
		if err := run.acceptContextReadings(a, c, []core.Reading{r}); err != nil {
			return e, err
		}
	}
	stamp := &core.ContextStamp{Text: contextStampText(r)}
	for _, note := range a.ContextBands.Pending {
		stamp.Crossings = append(stamp.Crossings, note.Envelope.ID)
		// The crossing owns its evaluated threshold and rendered action. A later
		// delivery may load a different collar; it cannot relabel this event.
		action := note.Envelope.Message.Text
		if action == "" {
			action = fmt.Sprintf("%s crossed: save state and compact/clear", note.Band)
		}
		stamp.Text += "; " + action
	}
	e.ContextStamp = stamp
	// The receipt owns the submitted wire, including when the process exits
	// between publication and recording input_started.
	if err := l.Paths.Publish(e); err != nil {
		return e, err
	}
	if a.Clear != nil && a.Clear.Envelope.ID == e.ID {
		a.Clear.Envelope = e
	}
	return e, l.Save(*a)
}

func (run *runtime) settleContextStamp(l *store.LockedAgent, a *core.Agent, e core.Envelope, outcome string) error {
	if e.ContextStamp == nil || outcome != "delivered" {
		return nil
	}
	kept := a.ContextBands.Pending[:0]
	for _, note := range a.ContextBands.Pending {
		if slices.Contains(e.ContextStamp.Crossings, note.Envelope.ID) {
			if note.Envelope.ID != e.ID {
				if err := l.Withdraw(note.Envelope.ID); err != nil && !errors.Is(err, os.ErrNotExist) {
					return err
				}
			}
			continue
		}
		kept = append(kept, note)
	}
	a.ContextBands.Pending = kept
	return l.Save(*a)
}

// Native admission compares the exact retained continuation, never a fresh stamp.
func (run *runtime) retainedResume(a core.Agent) (core.Envelope, error) {
	c := a.Compaction
	p, err := run.team.Agent(a.ID)
	if err != nil {
		return core.Envelope{}, err
	}
	id := core.EnvelopeID("resume-" + c.ID)
	for _, dir := range []string{"new", "cur", "failed"} {
		e, err := p.ReadEnvelope(dir, id)
		if err == nil {
			return e, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return core.Envelope{}, err
		}
	}
	return core.Envelope{ID: id, Token: c.ResumeToken, From: c.ResumeFrom, Message: c.Resume, Purpose: "resume"}, nil
}
