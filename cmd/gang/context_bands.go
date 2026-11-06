package main

import (
	"fmt"
	"strings"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/store"
	"github.com/adambiggs/gangline/substrate"
)

const contextBandCompactCommand = "gang compact --note 'Resume from FILE'"

const contextBandAdvice = "{{band}} {{threshold_percent}}% crossed: save state and compact/clear"
const contextBandOrder = "{{band}} {{threshold_percent}}% crossed: save state and compact/clear now"

func renderContextBandMessage(band harness.ContextBand, last bool, a *core.Agent, r core.Reading) string {
	message := band.Message
	if message == "" {
		message = contextBandAdvice
		if last {
			message = contextBandOrder
		}
	}
	return strings.NewReplacer(
		"{{band}}", band.Name,
		"{{threshold_percent}}", fmt.Sprintf("%.0f", band.At*100),
		"{{used_tokens}}", fmt.Sprint(*r.Used),
		"{{limit_tokens}}", fmt.Sprint(*r.Limit),
		"{{used_percent}}", fmt.Sprintf("%.0f", *r.Percent),
		"{{model}}", r.Model,
		"{{agent_name}}", string(a.Name),
		"{{compact_command}}", contextBandCompactCommand,
	).Replace(message)
}

// Accept each reading separately: a transcript batch can cross several bands,
// compact, then cross them again. Save these intents with the native cursor.
func (run *runtime) acceptContextReadings(a *core.Agent, c harness.Collar, readings []core.Reading) error {
	for _, r := range readings {
		if pending := a.Compaction; pending != nil && (pending.Status == "submitted" || pending.Status == "unverified") && r.Kind == "compaction-finished" && r.At != nil && r.At.After(pending.StartedAt) {
			pending.CompletedAt = *r.At
		}
		finished := r.Kind == "compaction-finished" && r.At != nil && r.At.After(a.Native.ConfirmedCompactedAt)
		if finished {
			a.ContextBands.Pending = nil
			a.ContextBands.DiscardQueued = true
		}
		acceptReadings(&a.Native, []core.Reading{r})
		if r.Kind == "compaction-finished" {
			continue
		}
		if err := run.noteContextBands(a, c); err != nil {
			return err
		}
	}
	return nil
}

func (run *runtime) noteContextBands(a *core.Agent, c harness.Collar) error {
	if a.Status != core.Active {
		return nil
	}
	state := &a.ContextBands
	c = agentContextCollar(*a, c)
	if a.Native.CompactedAt.After(state.CompactedAt) {
		state.Model, state.Percent, state.CompactedAt = "", 0, a.Native.CompactedAt
	}
	r := a.Native.Context
	if r.Status != "observed" || r.Model == "" || r.Used == nil || r.Limit == nil || r.Percent == nil {
		return nil
	}
	previous := state.Percent
	switch {
	case state.Model == "" && state.CompactedAt.IsZero() || state.Model != "" && state.Model != r.Model:
		previous = -1
	// The first reading after compaction is the new baseline. A note asks
	// the agent to compact, so it never repeats for the context it just
	// compacted to.
	case state.Model == "":
		previous = *r.Percent
	}
	last := harness.ActiveContextBand(c, r.Model, harness.ContextReading{Percent: 1})
	for _, band := range harness.CrossedContextBands(c, r.Model, previous/100, *r.Percent/100) {
		token, err := randomEnvelopeToken()
		if err != nil {
			return err
		}
		state.Sequence++
		e := core.Envelope{
			ID: core.EnvelopeID(fmt.Sprintf("context-%d", state.Sequence)), Token: token, Recipient: a.ID, To: a.Name,
			From: core.Sender{Kind: core.SenderGangline, Name: "context-band"}, CreatedAt: run.cmd.now(),
			Message: core.Message{Text: renderContextBandMessage(band, last != nil && band.Name == last.Name, a, r)},
		}
		state.Pending = append(state.Pending, core.ContextBandNote{Threshold: band.At, Band: band.Name, Reading: r, Envelope: e})
		if err := run.record(*a, core.Event{Type: "context_band_crossed", ID: string(e.ID), Status: band.Name, Envelope: &e, Readings: []core.Reading{r}}); err != nil {
			return err
		}
	}
	state.Model, state.Percent = r.Model, *r.Percent
	return nil
}

// Pending intents are saved before publication, and cleared before delivery.
// Recovery therefore reuses the same inbox identity and never types twice.
func (run *runtime) publishContextNotes(l *store.LockedAgent, a *core.Agent) error {
	if a.ContextBands.DiscardQueued {
		if err := l.Save(*a); err != nil {
			return err
		}
		queued, err := l.Paths.ListNew()
		if err != nil {
			return err
		}
		for _, e := range queued {
			if !isContextBandNotice(e) {
				// Unsubmitted mail survives the reset, with a new delivery-time
				// observation. Receipts of possibly submitted input stay frozen.
				if e.ContextStamp != nil && len(e.ContextStamp.Crossings) > 0 && e.PasteOnly == nil {
					e.ContextStamp = nil
					if err := l.Paths.Publish(e); err != nil {
						return err
					}
				}
				continue
			}
			const reason = "context band was queued before the native context reset"
			if err := l.Withdraw(e.ID); err != nil {
				return err
			}
			if err := run.record(*a, core.Event{Type: "send_cancelled", ID: string(e.ID), Reason: reason}); err != nil {
				return err
			}
		}
		a.ContextBands.DiscardQueued = false
		if err := l.Save(*a); err != nil {
			return err
		}
	}
	for _, note := range a.ContextBands.Pending {
		if run.cmd.now().Sub(note.Envelope.CreatedAt) < watchdogTimeout {
			continue
		}
		// Ordinary mail gets one tick to carry the crossing. A quiet recipient
		// receives one stamp-only fallback through the same guarded inbox.
		if err := run.publishOnce(l, a, note.Envelope); err != nil {
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
			if err := run.acceptContextReadings(a, c, []core.Reading{{Kind: "context", Source: "screen", Status: "observed", At: &at, Model: model, Used: &reading.Used, Limit: &reading.Limit, Percent: &percent}}); err != nil {
				return err
			}
		}
	}
	if err := run.noteContextBands(a, c); err != nil {
		return err
	}
	if err := l.Save(*a); err != nil {
		return err
	}
	return run.publishContextNotes(l, a)
}
