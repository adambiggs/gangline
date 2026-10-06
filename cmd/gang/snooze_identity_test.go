package main

import (
	"strings"
	"testing"
)

func TestWakeExactSubmitWithoutTurnIdentityRemainsPending(t *testing.T) {
	f, a, p := claudeRecipient(t)
	f.env["GANGLINE_HITCH_ID"] = string(a.ID)
	key := string(a.ID)
	if err := f.run.withUsageState(func(state *usageState) error {
		state.Snoozes[key] = usageSnooze{ID: "wake-no-turn", Token: "0123456789abcdef", CallerID: a.ID, CallerName: a.Name, At: f.cmd.now(), Note: "resume saved work"}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	// The native submit hook maps this exact prompt and session but receives
	// no prompt_id. Its receipt still proves delivery.
	if err := f.run.flushUsageWork(); err != nil {
		t.Fatal(err)
	}
	recent := usageSnapshot(t, f.run).Recent[key]
	if recent.ID != "wake-no-turn" || recent.TurnID != "" || recent.SubmittedAt.IsZero() || f.input.submits != 1 {
		t.Fatalf("wake without turn identity: %+v submits=%d", recent, f.input.submits)
	}
	w, err := p.ReadWitness()
	if err != nil || w.TurnID != "" || !strings.Contains(w.Prompt, "resume saved work") {
		t.Fatalf("submit receipt: %+v %v", w, err)
	}
	c, err := loadCollar("claude", f.run.settings)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.run.observeSnoozeTurn(a, c.Primitives.TurnBoundary, hookNotice{Kind: "turn-finished", TurnID: "different-turn"}, false); err != nil {
		t.Fatal(err)
	}
	if got := usageSnapshot(t, f.run).Recent[key]; got.ID != recent.ID {
		t.Fatalf("unattributed success cleared wake: %+v", got)
	}
	if got := snoozeStatusText(recent, false); got != "native turn identity unavailable" {
		t.Fatalf("missing identity status = %q", got)
	}
	// Both the caller and the lead use the retained wake formatter.
	if got := teammateWakeRow(recent, true, "lead"); !strings.Contains(got, "native turn identity unavailable") {
		t.Fatalf("lead wake status = %q", got)
	}
}
