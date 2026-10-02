package main

import (
	"strings"
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/substrate"
)

func failedTurnCompactFixture(t *testing.T, failure string) (*stateFixture, core.Agent) {
	t.Helper()
	f, a := compactStartFixture(t, compactDraftScreen, func(c harness.Collar) substrate.Screen { return claudeCompactingScreen(t, c) })
	return f, f.setAgent(t, *a, func(a *core.Agent) {
		a.Activity, a.Evidence = core.Unknown, "native turn failed: "+failure
		a.Native.TurnFailure = failure
	})
}

func readAgent(t *testing.T, f *stateFixture, id core.HitchID) core.Agent {
	t.Helper()
	p, err := f.run.team.Agent(id)
	if err != nil {
		t.Fatal(err)
	}
	got, err := p.Read()
	if err != nil {
		t.Fatal(err)
	}
	return got
}

// A failed turn leaves the activity unknown, but an idle screen after a
// failure the provider did not cause can take the compaction that lets the
// agent recover.
func TestCompactionStartsAfterNonCapacityTurnFailure(t *testing.T) {
	for _, failure := range []string{"unknown", "invalid_request: fixture rejected this request", "native failure without turn identity: unknown", "native turn failed"} {
		t.Run(failure, func(t *testing.T) {
			f, a := failedTurnCompactFixture(t, failure)
			_ = f.cmd.compact([]string{"--resume", "Resume from the state file."})
			got := readAgent(t, f, a.ID)
			if f.input.submits == 0 || got.Compaction.Status != "submitted" {
				t.Fatalf("compaction did not start: submits=%d compaction=%+v", f.input.submits, got.Compaction)
			}
			if n := eventsOfType(t, f, "compaction_waiting"); n != 0 {
				t.Fatalf("logged %d capacity waits for a non-capacity failure", n)
			}
		})
	}
}

// A capacity failure would fail the compaction the same way, so it waits; the
// wait is logged once, shows in the command's output, and ends when the
// failure clears.
func TestCompactionWaitsOnCapacityTurnFailure(t *testing.T) {
	for _, tc := range []struct{ failure, class string }{
		{"rate_limit", "rate_limit"},
		{"overloaded", "overloaded"},
		{"server_error: upstream detail", "server_error"},
		{"native failure without turn identity: billing_error", "billing_error"},
	} {
		t.Run(tc.failure, func(t *testing.T) {
			f, a := failedTurnCompactFixture(t, tc.failure)
			if err := f.cmd.compact([]string{"--resume", "Resume from the state file."}); err != nil {
				t.Fatal(err)
			}
			want := "provider capacity class " + tc.class
			if out := f.out.String(); !strings.Contains(out, "queued; native turn failed with "+want) {
				t.Fatalf("output does not name the capacity wait: %q", out)
			}
			for range 2 {
				if err := f.cmd.tick([]string{"--agent", "worker"}); err != nil {
					t.Fatal(err)
				}
			}
			got := readAgent(t, f, a.ID)
			if f.input.submits != 0 || got.Compaction.Status != "queued" || !strings.Contains(got.Compaction.Reason, want) {
				t.Fatalf("submits=%d compaction=%+v", f.input.submits, got.Compaction)
			}
			if n := eventsOfType(t, f, "compaction_waiting"); n != 1 {
				t.Fatalf("logged %d capacity waits across three observations, want 1", n)
			}
			f.setAgent(t, got, func(a *core.Agent) { a.Native.TurnFailure = "" })
			if err := f.cmd.tick([]string{"--agent", "worker"}); err != nil {
				t.Fatal(err)
			}
			got = readAgent(t, f, a.ID)
			if f.input.submits == 0 || got.Compaction.Status == "queued" {
				t.Fatalf("compaction did not start once the failure cleared: submits=%d compaction=%+v", f.input.submits, got.Compaction)
			}
			if n := eventsOfType(t, f, "compaction_waiting"); n != 2 {
				t.Fatalf("logged %d capacity wait events, want the wait and its end", n)
			}
		})
	}
}

// Claude keeps its final 529 error on screen after the turn fails, and the
// collar reads that line as a retry in progress, so the screen is not idle; the
// capacity wait must still be logged and shown.
func TestCompactionCapacityWaitShownWhileScreenReadsBusy(t *testing.T) {
	f, a := failedTurnCompactFixture(t, "server_error")
	c, err := loadCollar("claude", f.run.settings)
	if err != nil {
		t.Fatal(err)
	}
	f.input.screen = screenWithText("⏺ API Error: 529 Overloaded. This is a server-side issue, usually", "  temporary — try again in a moment.", "", "────────", "❯ ", "────────")
	if busy, err := harness.Busy(c, f.input.screen); err != nil || !busy {
		t.Fatalf("failed-turn screen premise: busy=%v err=%v", busy, err)
	}
	if err := f.cmd.compact([]string{"--resume", "Resume from the state file."}); err != nil {
		t.Fatal(err)
	}
	want := "native turn failed with provider capacity class server_error"
	if out := f.out.String(); !strings.Contains(out, "queued; "+want) {
		t.Fatalf("output does not name the capacity wait: %q", out)
	}
	got := readAgent(t, f, a.ID)
	if f.input.submits != 0 || got.Compaction.Status != "queued" || !strings.Contains(got.Compaction.Reason, want) {
		t.Fatalf("submits=%d compaction=%+v", f.input.submits, got.Compaction)
	}
	if n := eventsOfType(t, f, "compaction_waiting"); n != 1 {
		t.Fatalf("logged %d capacity waits, want 1", n)
	}
}

// Claude paints no spinner while a reply streams, so an idle-looking screen
// after a failure is not idle while a later submitted turn is still open.
func TestCompactionAfterTurnFailureWaitsForOpenTurn(t *testing.T) {
	f, a := failedTurnCompactFixture(t, "unknown")
	f.setAgent(t, a, func(a *core.Agent) {
		a.Native.SubmittedAt = time.Now()
		a.ScreenFingerprint = "an earlier screen"
	})
	if err := f.cmd.compact([]string{"--resume", "Resume from the state file."}); err != nil {
		t.Fatal(err)
	}
	got := readAgent(t, f, a.ID)
	if f.input.submits != 0 || got.Compaction.Status != "queued" {
		t.Fatalf("compaction started behind an open turn: submits=%d compaction=%+v", f.input.submits, got.Compaction)
	}
}
