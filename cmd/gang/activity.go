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
	if found {
		activity, evidence = core.Blocked, blocked.Evidence
	} else {
		idle, err := harness.Idle(c, screen)
		if err != nil {
			activity, evidence = core.Unknown, err.Error()
		} else if idle {
			activity, evidence = core.Idle, ""
		} else if busy, err := harness.Busy(c, screen); err != nil {
			activity, evidence = core.Unknown, err.Error()
		} else if !busy {
			activity, evidence = core.Blocked, "native composer contains unsubmitted input"
		} else if a.InterruptDeadline.IsZero() {
			activity, evidence = core.Busy, ""
		}
	}
	if a.Compaction != nil && a.Compaction.Status == "submitted" && activity != core.Blocked {
		activity, evidence = core.Compacting, "native compaction completion unconfirmed; resume follows confirmed completion"
	}
	if a.Native.TurnFailure != "" {
		activity, evidence = core.Unknown, "native turn failed: "+a.Native.TurnFailure
	}
	fingerprint := harness.ScreenFingerprint(screen)
	if a.ScreenFingerprint != fingerprint {
		a.ScreenFingerprint, a.ScreenSince = fingerprint, run.cmd.now()
		if err := l.Save(*a); err != nil {
			return err
		}
	}
	wedge, err := harness.DetectWedge(c.Primitives.Wedge, harness.WedgeObservation{
		Previous: a.ScreenFingerprint, Current: screen, BusySince: a.ScreenSince,
		ObservedAt: run.cmd.now(), TurnActive: activity == core.Busy,
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
