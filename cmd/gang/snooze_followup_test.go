//go:build acceptance

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
)

func TestWakeFollowupRetainsBothOutcomes(t *testing.T) {
	for _, outcome := range []string{"success", "failure", "cap", "cap-reset"} {
		for _, overdue := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/overdue=%t", outcome, overdue), func(t *testing.T) {
				f := newStateFixture(t)
				a := f.add(t, "worker-id", "worker", "codex")
				key := string(a.ID)
				f.env["GANGLINE_HITCH_ID"], f.env["GANG_AGENT_ID"], f.env["TMUX_PANE"] = key, key, a.Pane
				start := f.cmd.now()
				turn := "first-turn"
				f.input.submit = func(prompt string) error {
					payload, err := json.Marshal(map[string]string{"hook_event_name": "UserPromptSubmit", "prompt": prompt, "session_id": "s", "transcript_path": f.transcript, "turn_id": turn})
					if err != nil {
						return err
					}
					hook := f.cmd
					hook.stdin = bytes.NewReader(payload)
					return hook.hook(nil)
				}
				schedule := func(at string) usageSnooze {
					t.Helper()
					if err := f.cmd.snooze([]string{"--at", at}); err != nil {
						t.Fatal(err)
					}
					return usageSnapshot(t, f.run).Snoozes[key]
				}
				flush := func() {
					t.Helper()
					if err := f.run.flushUsageWork(); err != nil {
						t.Fatal(err)
					}
				}
				first := schedule("1m")
				setClock(f, start.Add(time.Minute))
				flush()
				if recent := usageSnapshot(t, f.run).Recent[key]; recent.ID != first.ID || recent.TurnID != turn || f.input.submits != 1 {
					t.Fatalf("first wake not delivered with turn: %+v submits=%d", recent, f.input.submits)
				}
				replaced := schedule("10m")
				next := schedule("15m")
				state := usageSnapshot(t, f.run)
				if state.Recent[key].ID != first.ID || state.Snoozes[key].ID != next.ID {
					t.Fatalf("scheduling lost wake: %+v", state)
				}
				f.out.Reset()
				if err := f.cmd.snooze([]string{"--status"}); err != nil {
					t.Fatal(err)
				}
				for _, id := range []core.EnvelopeID{first.ID, next.ID} {
					if strings.Count(f.out.String(), string(id)) != 1 {
						t.Fatalf("status missing wake %s: %s", id, f.out.String())
					}
				}
				if overdue {
					setClock(f, next.At.Add(time.Second))
					flush()
					flush()
					state = usageSnapshot(t, f.run)
					if f.input.submits != 1 || state.Recent[key].ID != first.ID || state.Snoozes[key].ID != next.ID || len(inboxNew(t, f, a)) != 0 {
						t.Fatalf("overdue follow-up overtook active wake: %+v submits=%d", state, f.input.submits)
					}
				}
				notice := hookNotice{Kind: "turn-finished", TurnID: turn}
				event := "snooze_completed"
				if outcome != "success" {
					notice.Kind, notice.Failure = "turn-failed", "Login expired"
					a.Native.FailedTurn = turn
					event = "snooze_failed"
					if strings.HasPrefix(outcome, "cap") {
						notice.Failure, event = "Usage limit reached", "snooze_cap_rejected"
					}
				}
				if outcome == "cap-reset" {
					at := f.cmd.now()
					a.Native.Limits = core.Reading{Status: "observed", At: &at, Limits: []core.LimitWindow{{Label: "five_hour", UsedPercent: 100, ResetAt: at.Add(time.Hour).Unix()}}}
				}
				for range 2 {
					if err := f.run.observeSnoozeTurn(a, harness.Invocation{}, nil, notice, false); err != nil {
						t.Fatal(err)
					}
				}
				state = usageSnapshot(t, f.run)
				if state.Snoozes[key].ID != next.ID {
					t.Fatalf("outcome discarded follow-up: %+v", state)
				}
				summary := wakeEventSummary(wakeEvents(t, f, "worker"))
				if strings.Count(summary, event+" "+string(first.ID)) != 1 || outcome != "cap-reset" && strings.Contains(summary, "snooze_cleared "+string(first.ID)) || !strings.Contains(summary, "snooze_cleared "+string(replaced.ID)) {
					t.Fatalf("first outcome or replacement history lost: %s", summary)
				}
				turn = "followup-turn"
				if !overdue {
					flush()
					if f.input.submits != 1 {
						t.Fatal("follow-up delivered before due")
					}
					setClock(f, next.At)
				}
				flush()
				flush()
				state = usageSnapshot(t, f.run)
				if f.input.submits != 2 || state.Snoozes[key].ID != "" || state.Recent[key].ID != next.ID || state.Recent[key].TurnID != turn {
					t.Fatalf("follow-up not delivered exactly once: %+v submits=%d", state, f.input.submits)
				}
				a.Native.FailedTurn = ""
				if err := f.run.observeSnoozeTurn(a, harness.Invocation{}, nil, hookNotice{Kind: "turn-finished", TurnID: turn}, false); err != nil {
					t.Fatal(err)
				}
				flush()
				summary = wakeEventSummary(wakeEvents(t, f, "worker"))
				if strings.Count(summary, event+" "+string(first.ID)) != 1 || strings.Count(summary, "snooze_completed "+string(next.ID)) != 1 || f.input.submits != 2 {
					t.Fatalf("both wake outcomes not retained: %s; submits=%d", summary, f.input.submits)
				}
			})
		}
	}
}

func TestWakeFollowupKeepsSubmittedWakeGuard(t *testing.T) {
	for _, submission := range []string{"accepted", "unverified"} {
		t.Run(submission, func(t *testing.T) {
			f := newStateFixture(t)
			a := f.add(t, "worker-id", "worker", "codex")
			key := string(a.ID)
			f.env["GANGLINE_HITCH_ID"], f.env["GANG_AGENT_ID"], f.env["TMUX_PANE"] = key, key, a.Pane
			pending := usageSnooze{ID: "submitted", CallerID: a.ID, RecipientID: a.ID, Submission: submission}
			if err := f.run.withUsageState(func(state *usageState) error { state.Snoozes[key] = pending; return nil }); err != nil {
				t.Fatal(err)
			}
			if err := f.cmd.snooze([]string{"--at", "15m"}); err == nil || !strings.Contains(err.Error(), "already submitted") {
				t.Fatalf("submitted wake replacement: %v", err)
			}
			if state := usageSnapshot(t, f.run); state.Snoozes[key] != pending || len(state.Recent) != 0 {
				t.Fatalf("refusal changed state: %+v", state)
			}
		})
	}
}

func TestWakeFollowupSurvivesRecipientDisappearance(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "worker-id", "worker", "codex")
	lead := f.add(t, "lead-id", "lead", "codex")
	saveAgent(t, f, lead.ID, func(a *core.Agent) { a.Role = "lead" })
	key := string(a.ID)
	f.env["GANGLINE_HITCH_ID"], f.env["GANG_AGENT_ID"], f.env["TMUX_PANE"] = key, key, a.Pane
	now := f.cmd.now()
	if err := f.run.withUsageState(func(state *usageState) error {
		state.Recent[key] = usageSnooze{ID: "active-wake", CallerID: a.ID, CallerName: a.Name, RecipientID: a.ID, RecipientName: a.Name, TurnID: "active-turn", At: now}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := f.cmd.snooze([]string{"--at", "15m"}); err != nil {
		t.Fatal(err)
	}
	next := usageSnapshot(t, f.run).Snoozes[key]
	if err := f.run.team.RemoveName(a.Name, a.ID); err != nil {
		t.Fatal(err)
	}
	if err := f.run.flushUsageWork(); err != nil {
		t.Fatal(err)
	}
	if state := usageSnapshot(t, f.run); state.Snoozes[key] != next || len(state.Recent) != 0 || f.input.submits != 0 {
		t.Fatalf("disappearance lost follow-up: %+v", state)
	}
	events := wakeEvents(t, f, "worker")
	if len(events) != 2 || events[1].Type != "snooze_cleared" || events[1].ID != "active-wake" || !strings.Contains(events[1].Reason, string(next.ID)) {
		t.Fatalf("disappearance history: %+v", events)
	}
	f.env["GANGLINE_HITCH_ID"] = string(lead.ID)
	setClock(f, next.At)
	if err := f.run.flushUsageWork(); err != nil {
		t.Fatal(err)
	}
	if state := usageSnapshot(t, f.run); state.Recent[key].ID != next.ID || state.Recent[key].RecipientID != lead.ID || f.input.submits != 1 {
		t.Fatalf("follow-up not routed to lead: %+v", state)
	}
}
