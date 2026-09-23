package main

import (
	"fmt"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/store"
	"github.com/adambiggs/gangline/substrate"
)

// Accept each reading separately: a transcript batch can cross several bands,
// compact, then cross them again. Save these intents with the native cursor.
func (run *runtime) acceptContextReadings(a *core.Agent, c harness.Collar, readings []core.Reading) {
	for _, r := range readings {
		acceptReadings(&a.Native, []core.Reading{r})
		run.noteContextBands(a, c)
	}
}

func (run *runtime) noteContextBands(a *core.Agent, c harness.Collar) {
	if a.Status != core.Active {
		return
	}
	state := &a.ContextBands
	if a.Native.CompactedAt.After(state.CompactedAt) {
		state.Model, state.Percent, state.CompactedAt = "", 0, a.Native.CompactedAt
	}
	r := a.Native.Context
	if r.Status != "observed" || r.Model == "" || r.Used == nil || r.Limit == nil || r.Percent == nil {
		return
	}
	previous := state.Percent
	if state.Model != r.Model {
		previous = -1
	}
	for _, band := range harness.CrossedContextBands(c, r.Model, previous/100, *r.Percent/100) {
		state.Sequence++
		e := core.Envelope{
			ID: core.EnvelopeID(fmt.Sprintf("context-%020d", state.Sequence)), Recipient: a.ID, To: a.Name,
			From: core.Sender{Kind: core.SenderSelfDeclared, Name: "context-band"}, CreatedAt: run.cmd.now(),
			Message: core.Message{Text: fmt.Sprintf("Context band %s crossed (threshold %.0f%%): %s, model %s. Follow your standing context-management instructions at the next natural checkpoint.", band.Name, band.At*100, contextUsageText(*r.Used, *r.Limit, *r.Percent), r.Model)},
		}
		state.Pending = append(state.Pending, core.ContextBandNote{Band: band.Name, Reading: r, Envelope: e})
	}
	state.Model, state.Percent = r.Model, *r.Percent
}

// Pending intents are saved before publication, and cleared before delivery.
// Recovery therefore reuses the same inbox identity and never types twice.
func (run *runtime) publishContextNotes(l *store.LockedAgent, a *core.Agent) error {
	for len(a.ContextBands.Pending) > 0 {
		note := a.ContextBands.Pending[0]
		if err := run.publishOnce(l, a, note.Envelope); err != nil {
			return err
		}
		if err := run.record(*a, core.Event{Type: "context_band_crossed", ID: string(note.Envelope.ID), Status: note.Band, Envelope: &note.Envelope, Readings: []core.Reading{note.Reading}}); err != nil {
			return err
		}
		a.ContextBands.Pending = a.ContextBands.Pending[1:]
		if err := l.Save(*a); err != nil {
			return err
		}
	}
	return nil
}

func (run *runtime) observeContextBands(l *store.LockedAgent, a *core.Agent, c harness.Collar, screen substrate.Screen) error {
	// A collar declaring native telemetry never falls back to the screen.
	if c.Primitives.Telemetry == nil {
		if reading, err := harness.ReadContext(c.Primitives.Context, screen); err == nil {
			model := ""
			if c.Models.Selected != nil {
				model, _ = harness.ReadSelectedModel(*c.Models.Selected, screen)
			}
			at, percent := run.cmd.now(), reading.Percent*100
			run.acceptContextReadings(a, c, []core.Reading{{Kind: "context", Source: "screen", Status: "observed", At: &at, Model: model, Used: &reading.Used, Limit: &reading.Limit, Percent: &percent}})
		}
	}
	run.noteContextBands(a, c)
	if err := l.Save(*a); err != nil {
		return err
	}
	return run.publishContextNotes(l, a)
}
