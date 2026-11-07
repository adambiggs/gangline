package main

import (
	"errors"
	"os"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/store"
)

// Confirm each native write independently; the ordinary receipt and witness
// deliberately retain only the latest result, so neither can represent a batch.
func (run *runtime) reconcileNativePending(l *store.LockedAgent, a *core.Agent) error {
	return run.reconcileNativePendingExcept(l, a, "")
}

func (run *runtime) reconcileNativePendingExcept(l *store.LockedAgent, a *core.Agent, synchronous core.EnvelopeID) error {
	if a.Input != nil {
		return nil
	}
	if err := l.Paths.RetireNativePending(a.Native.SessionID); err != nil {
		return err
	}
	pending, err := l.Paths.ListNativePending()
	if err != nil {
		return err
	}
	// Adopt the latest live hook through the normal boundary guard before
	// recording receipts. Old proofs never advance submission time or activity.
	for _, n := range pending {
		if n.Witness != nil && n.SessionID == a.Native.SessionID {
			c, err := loadCollar(a.Collar, run.settings)
			if err != nil {
				return err
			}
			if err := run.reconcileNativeBoundary(l, a, c, hookNotice{}); err != nil {
				return err
			}
			break
		}
	}
	for _, n := range pending {
		if n.Witness == nil {
			continue
		}
		w, e := *n.Witness, n.Envelope
		if w.SessionID != n.SessionID || w.Prompt != n.Prompt || w.At.Before(e.CreatedAt) {
			continue
		}
		// An exact historical submit proves this attempt even if a later native
		// session has taken over. Do not regress the current session or turn.
		// Pending evidence means settlement is incomplete even when the receipt
		// already says delivered: a prior owner may have exited before auditing it.
		// Retry all settlement steps; after a crash the audit may repeat an event,
		// but proof and sender notification must never disappear.
		if a.LastFailed == e.ID {
			a.LastFailed = ""
		}
		if err := run.settleReceipt(l, a, e, "delivered", "", w); err != nil {
			return err
		}
		current := w.SessionID == a.Native.SessionID && w.TurnID != "" && w.TurnID == a.Native.TurnID && w.At.Equal(a.Native.SubmittedAt)
		if current && a.Status == core.Active && a.Compaction == nil && a.Activity != core.Interrupting && a.Native.FinishedAt.Before(w.At) && a.Activity != core.Busy {
			basis := activityBasis(*a, "unread", "native-hook")
			if err := run.apply(l, a, core.Event{Type: "activity_observed", Activity: core.Busy, Reason: "native turn open: submit witnessed, no finish boundary yet", Basis: &basis}); err != nil {
				return err
			}
		}
		// Current ordinary input takes over the episode present before its write.
		// An old confirmation must not reset a later failure's recovery budget.
		if n.CapacityFingerprint != "" && a.Capacity.Fingerprint == n.CapacityFingerprint && current {
			if err := run.apply(l, a, core.Event{Type: "capacity_cleared"}); err != nil {
				return err
			}
		}
		// Retain evidence until the idempotent sender notice is published.
		if e.ID != synchronous && e.From.Kind == core.SenderAgent && e.From.HitchID != a.ID {
			if err := run.sendNotice(*a, e, "delivered"); err != nil {
				return err
			}
		}
		path, _ := l.Paths.EnvelopePath("failed", e.ID)
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if err := l.Paths.RemoveNativePending(e.ID); err != nil {
			return err
		}
	}
	return nil
}
