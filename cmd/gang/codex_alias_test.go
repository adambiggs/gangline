//go:build acceptance

package main

import (
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
)

func TestNativeErrorFallbackUsesDeclaredTelemetry(t *testing.T) {
	for _, tc := range []struct {
		name, collar, source string
		want                 bool
	}{
		{"codex", "codex", "codex", true},
		{"alternate", "codex-qwen", "codex", true},
		{"claude", "claude", "claude", false},
		{"name-without-codex-telemetry", "codex", "claude", false},
	} {
		for _, rearm := range []bool{false, true} {
			mode := "initial"
			if rearm {
				mode = "rearm"
			}
			t.Run(tc.name+"/"+mode, func(t *testing.T) {
				f := newStateFixture(t)
				c, err := harness.EmbeddedCollar(tc.source)
				if err != nil {
					t.Fatal(err)
				}
				c.Name = tc.collar
				a := f.add(t, "caller-id", "worker", tc.collar)
				at := f.cmd.now()
				reset := at.Add(5 * time.Hour)
				a.Native.SubmittedAt = at.Add(-time.Second)
				a.Native.TurnID = "wake-turn"
				a.Native.LastErrorAt, a.Native.LastError = at, "Usage limit reached"
				a.Native.Limits = core.Reading{Kind: "provider-limits", Status: "observed", At: &at, Limits: []core.LimitWindow{{Label: "five_hour", UsedPercent: 100, ResetAt: reset.Unix()}}}
				if rearm {
					if err := f.run.withUsageState(func(state *usageState) error {
						state.Recent[string(a.ID)] = usageSnooze{ID: "old-wake", CallerID: a.ID, CallerName: a.Name, RecipientID: a.ID, TurnID: "wake-turn", SubmittedAt: at.Add(-time.Second), Auto: true}
						return nil
					}); err != nil {
						t.Fatal(err)
					}
				}
				// No turn-failed notice: this exercises the native-error fallback through
				// the real outcome dispatcher with the loaded collar's telemetry.
				batch := statusHookBatch{notices: []hookNotice{{}}}
				if err := f.run.observeStatusOutcomes(a, c, batch, false); err != nil {
					t.Fatal(err)
				}
				next := usageSnapshot(t, f.run).Snoozes[string(a.ID)]
				if !tc.want {
					if next.ID != "" {
						t.Fatalf("non-Codex telemetry scheduled a wake: %+v", next)
					}
					return
				}
				expectedRearms := 0
				if rearm {
					expectedRearms = 1
				}
				if !next.Auto || !next.At.Equal(reset) || next.Rearms != expectedRearms {
					t.Fatalf("native error did not schedule expected wake: %+v", next)
				}
				if err := f.run.observeStatusOutcomes(a, c, batch, false); err != nil {
					t.Fatal(err)
				}
				if again := usageSnapshot(t, f.run).Snoozes[string(a.ID)]; again.ID != next.ID {
					t.Fatal("same native error scheduled a duplicate wake")
				}
			})
		}
	}
}
