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
	blocked, found, err := harness.InputBlocked(c, screen)
	if err != nil {
		return err
	}
	activity, evidence := a.Activity, a.Evidence
	fingerprint := harness.ScreenFingerprint(screen)
	screenBusy := false
	compacting, compactErr := harness.CompactionActive(c, screen)
	if found {
		activity, evidence = core.Blocked, blocked.Evidence
	} else if compactErr != nil {
		activity, evidence = core.Unknown, compactErr.Error()
	} else if compacting {
		if a.InterruptDeadline.IsZero() {
			activity, evidence = core.Compacting, "native compaction in progress"
		}
	} else {
		idle, err := harness.Idle(c, screen)
		if err != nil {
			activity, evidence = core.Unknown, err.Error()
		} else if idle {
			activity, evidence = core.Idle, ""
			open, err := run.openTurn(l, a, c, fingerprint)
			if err != nil {
				return err
			}
			if open {
				activity, evidence = core.Busy, "native turn open: submit witnessed, no finish boundary yet"
			}
		} else if busy, err := harness.Busy(c, screen); err != nil {
			activity, evidence = core.Unknown, err.Error()
		} else if !busy {
			activity, evidence = core.Blocked, "native composer contains unsubmitted input"
		} else if a.InterruptDeadline.IsZero() {
			activity, evidence, screenBusy = core.Busy, "", true
		}
	}
	if a.Compaction != nil && a.Compaction.Status == "submitted" && activity != core.Blocked {
		activity, evidence = core.Compacting, "native compaction completion unconfirmed; queued resume awaits confirmation"
	}
	if a.Native.TurnFailure != "" {
		activity, evidence = core.Unknown, "native turn failed: "+a.Native.TurnFailure
	}
	if a.ScreenFingerprint != fingerprint {
		a.ScreenFingerprint, a.ScreenSince = fingerprint, run.cmd.now()
		if err := l.Save(*a); err != nil {
			return err
		}
	}
	wedge, err := harness.DetectWedge(c.Primitives.Wedge, harness.WedgeObservation{
		Previous: a.ScreenFingerprint, Current: screen, BusySince: a.ScreenSince,
		ObservedAt: run.cmd.now(), TurnActive: screenBusy,
	})
	if err != nil {
		return err
	}
	if wedge.Detected {
		activity, evidence = core.Wedged, wedge.Evidence
	}
	if activity != a.Activity || evidence != a.Evidence {
		if err := run.apply(l, a, core.Event{Type: "activity_observed", Activity: activity, Reason: evidence}); err != nil {
			return err
		}
	}
	return nil
}

// openTurn reports whether a submitted turn is still running behind an
// idle-looking screen: Claude paints no spinner while a reply streams. A
// pending interrupt is excluded so that its completion can be observed.
func (run *runtime) openTurn(l *store.LockedAgent, a *core.Agent, c harness.Collar, fingerprint string) (bool, error) {
	quiet, err := harness.OpenTurnQuiet(c.Primitives.TurnBoundary)
	if err != nil || quiet == 0 || !a.InterruptDeadline.IsZero() || !a.Native.SubmittedAt.After(a.Native.FinishedAt) {
		return false, err
	}
	if a.ScreenFingerprint != fingerprint || run.cmd.now().Before(a.ScreenSince.Add(quiet)) {
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
	if err := run.apply(l, a, core.Event{Type: "activity_observed", Activity: core.Unknown, Reason: reason}); err != nil {
		return err
	}
	return run.mark(*a)
}
