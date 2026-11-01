package main

import (
	"context"
	"errors"
	"os"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/store"
	"github.com/adambiggs/gangline/substrate"
)

// Submit hooks can advance while a sweep holds the agent lock. Use the durable
// witness again before interpreting an error or sending its continuation.
func capacityTurn(l *store.LockedAgent, a core.Agent) (harness.CapacityTurn, error) {
	t := harness.CapacityTurn{SessionID: a.Native.SessionID, TurnID: a.Native.TurnID}
	w, err := l.Paths.ReadWitness()
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return t, err
	}
	if err == nil {
		t.SessionID, t.TurnID, t.Prompt = w.SessionID, w.TurnID, w.Prompt
	}
	if t.SessionID == a.Native.SessionID && t.TurnID == a.Native.FailedTurn {
		t.Failure = a.Native.TurnFailure
	}
	return t, nil
}

func capacityEnvelope(e core.Envelope) bool {
	return e.From.Kind == core.SenderGangline && e.From.Name == "capacity-recovery"
}

// A continuation can remain queued if a draft arrives between publication and
// delivery. It belongs to that failed turn alone, and requires idle input even
// for collars that accept ordinary messages during a running turn.
func (run *runtime) capacityInput(l *store.LockedAgent, a *core.Agent, b harnessInput, c harness.Collar, e core.Envelope) (ready, cancelled bool, err error) {
	screen, err := b.Capture(context.Background(), substrate.PaneID(a.Pane))
	if err != nil {
		return false, false, err
	}
	turn, err := capacityTurn(l, *a)
	if err != nil {
		return false, false, err
	}
	reason := ""
	if e.ID != core.EnvelopeID("capacity-"+a.Capacity.Fingerprint) || turn.Fingerprint() != "" && e.ID != core.EnvelopeID("capacity-"+turn.Fingerprint()) {
		reason = "capacity continuation superseded by another native turn"
	} else if !run.cmd.now().Before(a.Capacity.Deadline) {
		reason = "capacity recovery deadline elapsed before continuation delivery"
	}
	if reason != "" {
		if err := l.Withdraw(e.ID); err != nil {
			return false, false, err
		}
		return false, true, run.record(*a, core.Event{Type: "send_cancelled", ID: string(e.ID), Reason: reason})
	}
	capacity, found, err := harness.DetectCapacityForTurn(c, screen, turn)
	if errors.Is(err, harness.ErrCapacityIdentity) {
		return false, false, nil
	}
	return found && capacity.Fingerprint == a.Capacity.Fingerprint, false, err
}
