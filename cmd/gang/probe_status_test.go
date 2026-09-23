package main

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/substrate"
)

type failingActivityProbe struct {
	*inputFixture
	err error
}

func (p *failingActivityProbe) Capture(ctx context.Context, pane substrate.PaneID) (substrate.Screen, error) {
	if p.err != nil {
		return substrate.Screen{}, p.err
	}
	return p.inputFixture.Capture(ctx, pane)
}

func TestProbeTimeoutIsUnknownInsteadOfWedged(t *testing.T) {
	for _, read := range []string{"status", "roster", "tick"} {
		t.Run(read, func(t *testing.T) {
			f := newStateFixture(t)
			a := f.add(t, "a", "worker", "codex")
			p, _ := f.run.team.Agent(a.ID)
			a.Activity, a.Evidence = core.Wedged, "context deadline exceeded"
			f.input.screen = screenWithText("• Working (esc to interrupt)", "› ")
			a.ScreenFingerprint = harness.ScreenFingerprint(f.input.screen)
			a.ScreenSince = f.cmd.now().Add(-time.Hour)
			l, err := p.TryLock()
			if err != nil {
				t.Fatal(err)
			}
			if err := l.Save(a); err != nil {
				t.Fatal(err)
			}
			l.Close()
			probe := &failingActivityProbe{f.input, context.DeadlineExceeded}
			f.cmd.inputBackend = probe
			f.run.cmd = f.cmd
			switch read {
			case "status":
				err = f.cmd.status([]string{"worker", "--why"})
			case "roster":
				err = f.cmd.roster([]string{"--porcelain"})
			case "tick":
				err = f.run.tickAgent(a.ID, hookNotice{}, false)
			}
			if read == "tick" {
				if !errors.Is(err, context.DeadlineExceeded) {
					t.Fatalf("probe error lost: %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			got, err := p.Read()
			if err != nil {
				t.Fatal(err)
			}
			if got.Activity != core.Unknown || !strings.Contains(got.Evidence, "probe timed out") {
				t.Fatalf("probe failure: %+v", got)
			}
			if got.ScreenFingerprint != "" || !got.ScreenSince.IsZero() {
				t.Fatal("failed probe retained a continuous observation window")
			}
			if read != "tick" && (!strings.Contains(f.out.String(), "unknown") || strings.Contains(f.out.String(), "wedged")) {
				t.Fatalf("probe output: %q", f.out)
			}
			// A fresh identical screen after a gap cannot prove continuous wedging.
			probe.err = nil
			if err := f.cmd.status([]string{"worker", "--why"}); err != nil {
				t.Fatal(err)
			}
			got, err = p.Read()
			if err != nil || got.Activity != core.Busy {
				t.Fatalf("fresh busy observation: %+v %v", got, err)
			}
		})
	}
}

func TestLockedActivityProbeDoesNotPresentStaleWedge(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "codex")
	p, _ := f.run.team.Agent(a.ID)
	l, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	a.Activity, a.Evidence = core.Wedged, "context deadline exceeded"
	if err := l.Save(a); err != nil {
		t.Fatal(err)
	}
	if err := f.cmd.status([]string{"worker", "--why"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(f.out.String(), "unknown") || !strings.Contains(f.out.String(), "probe unavailable") || strings.Contains(f.out.String(), "wedged") {
		t.Fatalf("stale locked output: %q", f.out)
	}
}

func TestUnverifiedInputTimeoutDoesNotDiagnoseNativeWedge(t *testing.T) {
	a := core.Agent{ID: "a", Status: core.Active, Activity: core.Busy, Input: &core.InputIntent{ID: "m", Kind: "envelope"}}
	got, effects := core.Step(a, core.Event{Type: "input_finished", HitchID: a.ID, At: time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC), ID: "m", Status: "unverified", Reason: context.DeadlineExceeded.Error()})
	if len(effects) != 0 {
		t.Fatalf("test event rejected: %+v", effects)
	}
	if got.Activity != core.Unknown || !strings.Contains(got.Evidence, "input submission unverified") || got.Input != nil {
		t.Fatalf("timeout diagnosed native turn: %+v", got)
	}
}
