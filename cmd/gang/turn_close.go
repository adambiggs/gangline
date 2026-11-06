package main

import (
	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/store"
)

// observeTranscriptTurn closes the witnessed native turn, and completes an
// interrupt sent into it, once the harness transcript records the turn's end.
// Claude Code fires no hook when Escape ends a turn, so without the transcript
// the turn reads busy until a pane read finds the screen quiet for
// open_turn_quiet. An unreadable transcript leaves the turn to that fallback,
// as an unreadable queue does for a finish hook.
func (run *runtime) observeTranscriptTurn(l *store.LockedAgent, a *core.Agent, c harness.Collar) error {
	open := a.Native.SubmittedAt.After(a.Native.FinishedAt)
	interrupt := !a.InterruptDeadline.IsZero() && !compactionRunning(*a)
	if !open && !interrupt {
		return nil
	}
	end, closed, err := harness.TurnClosed(c.Primitives.TurnBoundary, a.Native.Transcript, a.Native.TurnID)
	if err != nil || !closed {
		return nil
	}
	// Only an end recorded after the first pending interrupt was sent shows
	// that it stopped something; an earlier end belongs to a turn already
	// over, and whatever runs now, such as a compaction the operator typed,
	// may not write one.
	interrupt = interrupt && !end.At.Before(a.InterruptAt)
	if !open && !interrupt {
		return nil
	}
	reason := "native transcript records the turn's end"
	if end.Interrupted {
		reason = "native transcript records the turn interrupted by the user"
	}
	if open {
		// The end follows the witnessed prompt, and gang records its own
		// delivery after the submit hook ran, so a fast turn can end before
		// SubmittedAt.
		a.Native.FinishedAt = end.At
		if a.Native.SubmittedAt.After(end.At) {
			a.Native.FinishedAt = a.Native.SubmittedAt
		}
		if err := l.Save(*a); err != nil {
			return err
		}
		// Dismissing a permission request with Escape fires no hook either.
		if w, pending, err := l.Paths.ReadPermissionWitness(); err != nil {
			return err
		} else if pending && !w.At.After(a.Native.FinishedAt) {
			if err := l.Paths.RemovePermissionWitness(); err != nil {
				return err
			}
		}
	}
	if interrupt {
		return run.apply(l, a, core.Event{Type: "interrupt_completed", Reason: reason})
	}
	// Only the turn's own readings end with it; a held draft, a failure or a
	// compaction keeps its reading until its own evidence changes.
	if a.Activity == core.Busy || a.Activity == core.Blocked && a.Evidence == permissionEvidence {
		basis := activityBasis(*a, "unread", "transcript-turn-end")
		return run.apply(l, a, core.Event{Type: "activity_observed", Activity: core.Idle, Reason: reason, Basis: &basis})
	}
	return run.record(*a, core.Event{Type: "observation", Reason: reason})
}

// compactionRunning reports a compaction the harness may be running: the end
// of the turn before it does not show that an interrupt stopped it.
func compactionRunning(a core.Agent) bool {
	return a.Compaction != nil && (a.Compaction.Status == "submitted" || a.Compaction.Status == "unverified")
}
