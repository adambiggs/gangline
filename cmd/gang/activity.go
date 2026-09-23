package main

import (
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
		} else {
			if a.InterruptDeadline.IsZero() {
				activity, evidence = core.Busy, ""
			}
		}
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
