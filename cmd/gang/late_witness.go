package main

import (
	"errors"
	"os"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/store"
)

// Retained uncertain and accepted receipts may become delivered on exact hook
// proof. Reconciliation never sends keys or infers full submission from a screen.
func (run *runtime) reconcileDelivery(l *store.LockedAgent, a *core.Agent) error {
	if (a.LastFailed == "" && a.LastAccepted == "") || a.Input != nil {
		return nil
	}
	w, err := l.Paths.ReadWitness()
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if a.Native.SessionID != "" && w.SessionID != a.Native.SessionID {
		return nil
	}
	c, err := loadCollar(a.Collar, run.settings)
	if err != nil {
		return err
	}
	for _, receipt := range []core.ResultRef{{ID: a.LastFailed, Directory: "failed"}, {ID: a.LastAccepted, Directory: "cur"}} {
		if receipt.ID == "" {
			continue
		}
		e, err := l.Paths.ReadEnvelope(receipt.Directory, receipt.ID)
		if err != nil {
			return err
		}
		if (e.Outcome != "unverified" && e.Outcome != "accepted") || w.At.Before(e.CreatedAt) {
			continue
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
			continue
		}
		if err := run.reopenReceipt(l, a, e, receipt.Directory); err != nil {
			return err
		}
		a.Native.SessionID, a.Native.TurnID, a.Native.Transcript = w.SessionID, w.TurnID, w.Transcript
		return run.finishInput(l, a, e, "delivered", "")
	}
	return nil
}

func (run *runtime) reopenUnverified(l *store.LockedAgent, a *core.Agent, e core.Envelope) error {
	return run.reopenReceipt(l, a, e, "failed")
}

func (run *runtime) reopenReceipt(l *store.LockedAgent, a *core.Agent, e core.Envelope, directory string) error {
	if err := run.apply(l, a, core.Event{Type: "input_started", ID: string(e.ID), Status: "envelope"}); err != nil {
		return err
	}
	from, _ := l.Paths.EnvelopePath(directory, e.ID)
	to, _ := l.Paths.EnvelopePath("new", e.ID)
	if err := os.Rename(from, to); err != nil {
		return err
	}
	if directory == "cur" {
		a.LastAccepted = ""
	} else {
		a.LastFailed = ""
	}
	return l.Save(*a)
}
