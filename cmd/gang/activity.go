package main

import (
	"context"
	"errors"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/store"
	"github.com/adambiggs/gangline/substrate"
)

// observeActivity uses current pane evidence without changing message receipts.
func (run *runtime) observeActivity(l *store.LockedAgent, a *core.Agent, c harness.Collar, screen substrate.Screen) error {
	_, err := run.observeScreen(l, a, c, screen)
	return err
}

// heldInputEvidence is the reading of a composer that holds input no submit key
// sent. Messages to the agent wait behind it.
const heldInputEvidence = "native composer contains unsubmitted input"
const submitPaintWindow = time.Second

// observeScreen is observeActivity that also reports whether the screen alone
// reads idle with no open turn, before a recorded turn failure or pending
// compaction overrides the activity.
func (run *runtime) observeScreen(l *store.LockedAgent, a *core.Agent, c harness.Collar, screen substrate.Screen) (screenIdle bool, err error) {
	blocked, found, err := harness.InputBlocked(c, screen)
	if err != nil {
		return false, err
	}
	activity, evidence := a.Activity, a.Evidence
	fingerprint := harness.ScreenFingerprint(screen)
	screenBusy := false
	compacting, compactErr := harness.CompactionActive(c, screen)
	basis := activityBasis(*a, "", "screen")
	if found {
		activity, evidence, basis.Screen = core.Blocked, blocked.Evidence, "blocked"
	} else if compactErr != nil {
		activity, evidence, basis.Screen = core.Unknown, compactErr.Error(), "unreadable"
	} else if compacting {
		basis.Screen = "compacting"
		if a.InterruptDeadline.IsZero() {
			activity, evidence = core.Compacting, "native compaction in progress"
		} else {
			basis.Rule = "interrupt-pending"
		}
	} else {
		idle, err := harness.Idle(c, screen)
		if err != nil {
			activity, evidence, basis.Screen = core.Unknown, err.Error(), "unreadable"
		} else if idle {
			activity, evidence, basis.Screen = core.Idle, "", "idle"
			open, err := run.openTurn(l, a, c, fingerprint)
			if err != nil {
				return false, err
			}
			if open {
				activity, evidence, basis.Rule = core.Busy, "native turn open: submit witnessed, no finish boundary yet", "open-turn"
			}
		} else if busy, err := harness.Busy(c, screen); err != nil {
			activity, evidence, basis.Screen = core.Unknown, err.Error(), "unreadable"
		} else if !busy {
			activity, evidence, basis.Screen = core.Blocked, heldInputEvidence, "unsubmitted"
			// A draft holds delivery while a spinnerless reply can still run.
			// Reconcile its turn too, so the display does not keep old work open.
			if open, err := run.openTurn(l, a, c, fingerprint); err != nil {
				return false, err
			} else if open {
				activity, basis.Rule = core.Busy, "open-turn"
			}
		} else {
			basis.Screen = "busy"
			if a.InterruptDeadline.IsZero() {
				activity, evidence, screenBusy = core.Busy, "", true
			} else {
				basis.Rule = "interrupt-pending"
			}
		}
	}
	if basis.Screen == "unreadable" || basis.Screen == "idle" {
		if _, pending, err := l.Paths.ReadPermissionWitness(); err != nil {
			return false, err
		} else if pending && basis.Screen == "unreadable" {
			activity, evidence, basis.Rule = core.Blocked, "native permission request awaits an answer", "permission-request"
		} else if pending {
			// An idle composer shows the request was dismissed, which fires no
			// hook. A busy screen does not: it can precede the prompt's render.
			if err := l.Paths.RemovePermissionWitness(); err != nil {
				return false, err
			}
		}
	}
	screenIdle = activity == core.Idle
	if a.Compaction != nil && a.Compaction.Status == "submitted" && activity != core.Blocked {
		activity, evidence, basis.Rule = core.Compacting, "native compaction completion unconfirmed; queued resume awaits confirmation", "compaction-record"
	}
	if a.Native.TurnFailure != "" {
		activity, evidence, basis.Rule = core.Unknown, "native turn failed: "+a.Native.TurnFailure, "turn-failure"
	}
	if a.ScreenFingerprint != fingerprint {
		a.ScreenFingerprint, a.ScreenSince = fingerprint, run.cmd.now()
		if err := l.Save(*a); err != nil {
			return false, err
		}
	}
	wedge, err := harness.DetectWedge(c.Primitives.Wedge, harness.WedgeObservation{
		Previous: a.ScreenFingerprint, Current: screen, BusySince: a.ScreenSince,
		ObservedAt: run.cmd.now(), TurnActive: screenBusy,
	})
	if err != nil {
		return false, err
	}
	if wedge.Detected {
		activity, evidence, basis.Rule = core.Wedged, wedge.Evidence, "wedge"
	}
	if activity != a.Activity || evidence != a.Evidence {
		if err := run.apply(l, a, core.Event{Type: "activity_observed", Activity: activity, Reason: evidence, Fingerprint: fingerprint, Basis: &basis}); err != nil {
			return false, err
		}
	}
	return screenIdle, nil
}

// activityBasis names what an activity reading of a was derived from: the
// collar's reading of the screen and the rule that set the activity.
func activityBasis(a core.Agent, screen, rule string) core.ActivityBasis {
	basis := core.ActivityBasis{Screen: screen, Rule: rule}
	if a.Compaction != nil {
		basis.Compaction = a.Compaction.Status
	}
	return basis
}

// openTurn reports whether a submitted turn is still running behind an
// idle-looking screen: Claude paints no spinner while a reply streams. A
// pending interrupt is excluded so that its completion can be observed.
func (run *runtime) openTurn(l *store.LockedAgent, a *core.Agent, c harness.Collar, fingerprint string) (bool, error) {
	quiet, err := harness.OpenTurnQuiet(c.Primitives.TurnBoundary)
	if err != nil || !a.InterruptDeadline.IsZero() || !a.Native.SubmittedAt.After(a.Native.FinishedAt) {
		return false, err
	}
	if quiet == 0 {
		// The submit hook precedes native UI paint. A short display grace
		// bridges that frame without inferring a native finish boundary.
		return run.cmd.now().Before(a.Native.SubmittedAt.Add(submitPaintWindow)), nil
	}
	quietSince := a.ScreenSince
	if a.Native.SubmittedAt.After(quietSince) {
		quietSince = a.Native.SubmittedAt
	}
	if a.ScreenFingerprint != fingerprint || run.cmd.now().Before(quietSince.Add(quiet)) {
		return true, nil
	}
	a.Native.FinishedAt = run.cmd.now()
	return false, l.Save(*a)
}

// A failed probe breaks the observation window; it cannot diagnose native work.
func (run *runtime) observeProbeFailure(l *store.LockedAgent, a *core.Agent, cause error) error {
	reason := "native activity probe failed: " + cause.Error()
	if errors.Is(cause, context.DeadlineExceeded) {
		reason = "native activity probe timed out: " + cause.Error()
	}
	if a.Native.TurnFailure != "" {
		reason = "native turn failed: " + a.Native.TurnFailure + "; " + reason
	}
	a.ScreenFingerprint, a.ScreenSince = "", time.Time{}
	if err := l.Save(*a); err != nil {
		return err
	}
	basis := activityBasis(*a, "unread", "probe-failure")
	if err := run.apply(l, a, core.Event{Type: "activity_observed", Activity: core.Unknown, Reason: reason, Basis: &basis}); err != nil {
		return err
	}
	return run.mark(*a)
}
