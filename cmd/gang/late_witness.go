package main

import (
	"errors"
	"os"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/store"
)

// Only the retained uncertain receipt can be reconciled. Native queue display
// alone is never a witness, and reconciliation never sends any keys.
func (run *runtime) reconcileDelivery(l *store.LockedAgent, a *core.Agent) error {
	if a.LastFailed == "" || a.Input != nil {
		return nil
	}
	e, err := l.Paths.ReadEnvelope("failed", a.LastFailed)
	if err != nil {
		return err
	}
	if e.Outcome != "unverified" {
		return nil
	}
	w, err := l.Paths.ReadWitness()
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if w.At.Before(e.CreatedAt) || (a.Native.SessionID != "" && w.SessionID != a.Native.SessionID) {
		return nil
	}
	c, err := loadCollar(a.Collar, run.settings)
	if err != nil {
		return err
	}
	wire, err := envelopeText(e)
	if err != nil {
		return err
	}
	matched, err := harness.SubmittedPromptMatches(c.Primitives.SubmitWitness, wire, w.Prompt)
	if err != nil {
		return err
	}
	if !matched {
		return nil
	}
	if err := run.reopenUnverified(l, a, e); err != nil {
		return err
	}
	a.Native.SessionID, a.Native.TurnID, a.Native.Transcript = w.SessionID, w.TurnID, w.Transcript
	return run.finishInput(l, a, e, "delivered", "")
}

func (run *runtime) reopenUnverified(l *store.LockedAgent, a *core.Agent, e core.Envelope) error {
	if err := run.apply(l, a, core.Event{Type: "input_started", ID: string(e.ID), Status: "envelope"}); err != nil {
		return err
	}
	from, _ := l.Paths.EnvelopePath("failed", e.ID)
	to, _ := l.Paths.EnvelopePath("new", e.ID)
	if err := os.Rename(from, to); err != nil {
		return err
	}
	a.LastFailed = ""
	return l.Save(*a)
}
