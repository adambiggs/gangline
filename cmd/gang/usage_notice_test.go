package main

import (
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
)

func TestUsageBandIdenticalCrossingsDeliverOnce(t *testing.T) {
	for _, collar := range []string{"claude", "codex"} {
		for _, window := range []string{"five_hour", "weekly"} {
			t.Run(collar+"/"+window, func(t *testing.T) {
				f := newStateFixture(t)
				lead := f.add(t, "lead-id", "lead", "codex")
				worker := f.add(t, "worker-id", "worker", collar)
				c, err := loadCollar(collar, f.run.settings)
				if err != nil {
					t.Fatal(err)
				}
				at := f.cmd.now()
				f.cmd.clock = func() time.Time { return at }
				f.run.cmd.clock = f.cmd.clock
				reset := at.Add(7 * 24 * time.Hour).Unix()
				observe := func(a core.Agent) {
					t.Helper()
					a.Native.Limits = core.Reading{Kind: "provider-limits", Status: "observed", At: &at, Limits: []core.LimitWindow{{Label: window, UsedPercent: 98, ResetAt: reset}}}
					if err := f.run.observeUsageBands(a, c); err != nil {
						t.Fatal(err)
					}
					if fired := usageSnapshot(t, f.run).Windows[collar+"/"+window].Fired; len(fired) != 2 {
						t.Fatalf("crossed bands = %v, want both recorded", fired)
					}
				}
				f.env["GANGLINE_HITCH_ID"] = string(lead.ID)
				observe(worker)
				if err := f.run.flushUsageWork(); err != nil {
					t.Fatal(err)
				}
				if f.input.submits != 1 {
					t.Fatalf("identical usage notices delivered %d times, want once; last = %q", f.input.submits, f.input.pasted)
				}
				if len(usageSnapshot(t, f.run).Notices) != 0 {
					t.Fatal("delivered notice remains pending")
				}
				// Reopening state and a newer reading from another agent must
				// retain both crossings after the single notice is acknowledged.
				f.run, err = f.cmd.runtime()
				if err != nil {
					t.Fatal(err)
				}
				at = at.Add(time.Second)
				observe(lead)
				if err := f.run.flushUsageWork(); err != nil {
					t.Fatal(err)
				}
				if f.input.submits != 1 {
					t.Fatal("same reset repeated a delivered crossing")
				}
				reset += int64((7 * 24 * time.Hour) / time.Second)
				at = at.Add(time.Second)
				observe(worker)
				if err := f.run.flushUsageWork(); err != nil {
					t.Fatal(err)
				}
				if f.input.submits != 2 {
					t.Fatalf("new reset delivered %d total notices, want two", f.input.submits)
				}
			})
		}
	}
}

func TestUsageBandDistinctGuidanceSurvivesCoalescing(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "codex")
	c, err := loadCollar("codex", f.run.settings)
	if err != nil {
		t.Fatal(err)
	}
	c.UsageBands["weekly"] = []harness.UsageBand{
		{Name: "yellow", At: 0.75},
		{Name: "orange", At: 0.85, Note: "save your work"},
		{Name: "red", At: 0.90},
	}
	at := f.cmd.now()
	a.Native.Limits = core.Reading{Kind: "provider-limits", Status: "observed", At: &at, Limits: []core.LimitWindow{{Label: "weekly", UsedPercent: 98, ResetAt: at.Add(7 * 24 * time.Hour).Unix()}}}
	if err := f.run.observeUsageBands(a, c); err != nil {
		t.Fatal(err)
	}
	state := usageSnapshot(t, f.run)
	if len(state.Notices) != 2 || len(state.Windows["codex/weekly"].Fired) != 3 {
		t.Fatalf("distinct guidance must survive while identical measurements coalesce: %+v", state)
	}
	if state.Notices[1].Text != state.Notices[0].Text+" save your work" {
		t.Fatalf("operator guidance lost: %+v", state.Notices)
	}
}
