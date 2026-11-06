package main

import (
	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/store"
)

// closeTranscriptTurn closes the witnessed native turn, and completes an
// interrupt sent into it, once the harness transcript records the turn's end.
// Claude Code fires no hook when Escape ends a turn, so without the transcript
// the turn stays open until the screen stays quiet for open_turn_quiet. An
// unreadable transcript leaves the turn to that fallback. It reports whether
// it found the turn ended.
func (run *runtime) closeTranscriptTurn(l *store.LockedAgent, a *core.Agent, c harness.Collar) (bool, error) {
	if !a.Native.SubmittedAt.After(a.Native.FinishedAt) {
		return false, nil
	}
	end, closed, err := harness.TurnClosed(c.Primitives.TurnBoundary, a.Native.Transcript, a.Native.TurnID)
	if err != nil || !closed {
		return false, nil
	}
	reason := "native transcript records the turn's end"
	if end.Interrupted {
		reason = "native transcript records the turn interrupted by the user"
	}
	// The end follows the witnessed prompt, and gang records its own delivery
	// after the submit hook ran, so a fast turn can end before SubmittedAt.
	a.Native.FinishedAt = end.At
	if a.Native.SubmittedAt.After(end.At) {
		a.Native.FinishedAt = a.Native.SubmittedAt
	}
	if err := l.Save(*a); err != nil {
		return false, err
	}
	// Dismissing a permission request with Escape fires no hook either.
	if w, pending, err := l.Paths.ReadPermissionWitness(); err != nil {
		return false, err
	} else if pending && !w.At.After(a.Native.FinishedAt) {
		if err := l.Paths.RemovePermissionWitness(); err != nil {
			return false, err
		}
	}
	if err := run.record(*a, core.Event{Type: "observation", Reason: reason}); err != nil {
		return false, err
	}
	// A compaction runs after the turn it follows ends, so the turn's end does
	// not show that the interrupt stopped it.
	if !a.InterruptDeadline.IsZero() && (a.Compaction == nil || a.Compaction.Status == "completed" || a.Compaction.Status == "failed") {
		if err := run.apply(l, a, core.Event{Type: "interrupt_completed", Reason: reason}); err != nil {
			return false, err
		}
	}
	return true, nil
}
