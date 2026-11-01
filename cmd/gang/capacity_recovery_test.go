//go:build acceptance

package main

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/store"
)

const capacityError = "Selected model is at capacity. Please try a different model."

// Native task_complete identifies the failed turn even when a tool's output
// has scrolled its originating prompt out of the pane. No Stop hook fires.
func TestCapacityRecoveryUsesRecordedTurnAfterToolOutput(t *testing.T) {
	for _, fresh := range []bool{false, true} {
		t.Run(map[bool]string{false: "expired hook", true: "fresh hook"}[fresh], func(t *testing.T) {
			f, a := codexTurnErrorFixture(t, "t1", "t1", "server_overloaded", capacityError)
			f.env["GANGLINE_HITCH_ID"] = string(a.ID)
			now := f.cmd.now()
			f.run.cmd.clock = func() time.Time { return now }
			if fresh {
				f.setAgent(t, a, func(a *core.Agent) {
					a.Native.HookKind, a.Native.HookAt = "activity", now
					a.Activity = core.Busy
				})
			}
			f.input.screen = screenWithText("• Ran set -euo pipefail", "  └ completed", "", "■ "+capacityError, "", "› ")
			if err := f.run.tickAgent(a.ID, hookNotice{}, false); err != nil {
				t.Fatal(err)
			}
			got := f.agent(t, a.ID)
			if got.Capacity.Fingerprint == "" || f.input.submits != 0 || got.Activity == core.Busy {
				t.Fatalf("failed turn not awaiting recovery: activity=%s capacity=%+v submits=%d", got.Activity, got.Capacity, f.input.submits)
			}
			now = got.Capacity.NextAt.Add(-time.Nanosecond)
			if err := f.run.tickAgent(a.ID, hookNotice{}, false); err != nil {
				t.Fatal(err)
			}
			if f.input.submits != 0 {
				t.Fatal("continuation submitted before backoff")
			}
			now = now.Add(time.Nanosecond)
			if err := f.run.tickAgent(a.ID, hookNotice{}, false); err != nil {
				t.Fatal(err)
			}
			if f.input.submits != 1 {
				t.Fatalf("continuation submits=%d", f.input.submits)
			}
		})
	}
}

func TestCapacityRecoveryRetainsBackoffAndDeadline(t *testing.T) {
	f, a := codexTurnErrorFixture(t, "t1", "t1", "server_overloaded", capacityError)
	p, _ := f.run.team.Agent(a.ID)
	now := f.cmd.now()
	f.run.cmd.clock = func() time.Time { return now }
	f.cmd.clock = f.run.cmd.clock
	f.input.submit = func(prompt string) error {
		return p.WriteWitness(store.Witness{ID: fmt.Sprintf("w%d", f.input.submits+1), At: now, SessionID: "s", TurnID: fmt.Sprintf("retry%d", f.input.submits), Transcript: a.Native.Transcript, Prompt: prompt})
	}
	idle := func() { f.input.screen = screenWithText("• Ran a tool", "", "■ "+capacityError, "", "› ") }
	tick := func() core.Agent {
		t.Helper()
		if err := f.run.tickAgent(a.ID, hookNotice{}, false); err != nil {
			t.Fatal(err)
		}
		return f.agent(t, a.ID)
	}
	idle()
	first := tick().Capacity
	for attempt := 1; attempt <= 2; attempt++ {
		before := f.agent(t, a.ID).Capacity
		if before.NextAt.Sub(now) != core.RetryDelay(attempt-1) {
			t.Fatalf("attempt %d delay=%s", attempt, before.NextAt.Sub(now))
		}
		now = before.NextAt.Add(-time.Nanosecond)
		tick()
		if f.input.submits != attempt-1 {
			t.Fatal("retry before fake deadline's 1ns margin")
		}
		now = now.Add(time.Nanosecond)
		tick()
		if f.input.submits != attempt {
			t.Fatalf("submits=%d want=%d", f.input.submits, attempt)
		}
		// The submitted turn now runs; it is not a successful recovery yet.
		now = now.Add(time.Second)
		f.input.screen = screenWithText("• Working (esc to interrupt)", "› ")
		busy := tick()
		if busy.Capacity.Attempts != attempt || !busy.Capacity.Deadline.Equal(first.Deadline) || busy.Activity != core.Busy {
			t.Fatalf("busy frame erased recovery: %+v activity=%s", busy.Capacity, busy.Activity)
		}
		if attempt == 2 {
			break
		}
		now = now.Add(time.Second)
		appendLines(t, a.Native.Transcript, fmt.Sprintf(`{"timestamp":%q,"type":"event_msg","payload":{"type":"task_complete","turn_id":"retry1","error":{"message":%q,"codex_error_info":"server_overloaded"}}}`, now.Format(time.RFC3339Nano), capacityError))
		idle()
		tick()
	}
	// Repeated observation cannot submit again while the last attempt runs,
	// even when its original recovery deadline has passed.
	now = first.Deadline
	if got := tick(); got.Activity != core.Busy || f.input.submits != 2 {
		t.Fatalf("running retry interrupted at expiry: activity=%s submits=%d", got.Activity, f.input.submits)
	}
	appendLines(t, a.Native.Transcript, fmt.Sprintf(`{"timestamp":%q,"type":"event_msg","payload":{"type":"task_complete","turn_id":"retry2","error":{"message":%q,"codex_error_info":"server_overloaded"}}}`, now.Format(time.RFC3339Nano), capacityError))
	idle()
	for range 2 {
		got := tick()
		if got.Activity != core.Wedged || got.Evidence != "provider capacity recovery deadline elapsed" || f.input.submits != 2 {
			t.Fatalf("expiry not retained: activity=%s evidence=%q submits=%d", got.Activity, got.Evidence, f.input.submits)
		}
	}
	if err := f.cmd.status([]string{"worker", "--why"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(f.out.String(), "provider capacity recovery deadline elapsed") {
		t.Fatalf("status hid expiry: %s", f.out.String())
	}
}

func TestCapacityRecoveryDoesNotReplayUnverifiedContinuation(t *testing.T) {
	f, a := codexTurnErrorFixture(t, "t1", "t1", "server_overloaded", capacityError)
	f.input.screen = screenWithText("■ "+capacityError, "", "› ")
	if err := f.run.tickAgent(a.ID, hookNotice{}, false); err != nil {
		t.Fatal(err)
	}
	capacity := f.agent(t, a.ID).Capacity
	f.run.cmd.clock = func() time.Time { return capacity.NextAt }
	f.input.submit = func(string) error { return errors.New("receipt interrupted") }
	for range 3 {
		if err := f.run.tickAgent(a.ID, hookNotice{}, false); err != nil {
			t.Fatal(err)
		}
		if got := f.agent(t, a.ID); got.Activity != core.Unknown || !strings.Contains(got.Evidence, "submission unverified") {
			t.Fatalf("uncertain continuation reported as settled: %s %q", got.Activity, got.Evidence)
		}
	}
	f.run.cmd.clock = func() time.Time { return capacity.Deadline }
	if err := f.run.tickAgent(a.ID, hookNotice{}, false); err != nil {
		t.Fatal(err)
	}
	f.cmd.clock = f.run.cmd.clock
	if err := f.cmd.status([]string{"worker", "--why"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(f.out.String(), "capacity continuation submission unverified") {
		t.Fatalf("status hid uncertain input: %s", f.out.String())
	}
	if f.input.submits != 1 {
		t.Fatalf("uncertain continuation replayed: submits=%d", f.input.submits)
	}
	p, _ := f.run.team.Agent(a.ID)
	receipt, err := p.ReadEnvelope("failed", core.EnvelopeID("capacity-"+capacity.Fingerprint))
	if err != nil || receipt.Outcome != "unverified" {
		t.Fatalf("uncertain receipt lost: %+v %v", receipt, err)
	}
}

func TestCapacityRecoveryDeliversQueuedWorkWithoutContinuation(t *testing.T) {
	f, a := codexTurnErrorFixture(t, "t1", "t1", "server_overloaded", capacityError)
	f.env["GANGLINE_HITCH_ID"] = string(a.ID)
	f.input.screen = screenWithText("■ "+capacityError, "", "› ")
	p, _ := f.run.team.Agent(a.ID)
	e := core.Envelope{ID: "queued-work", Token: "0123456789abcdef", Recipient: a.ID, To: a.Name, From: core.Sender{Kind: core.SenderSelfDeclared, Name: "operator"}, Message: core.Message{Text: "Continue with this correction."}, CreatedAt: f.cmd.now()}
	if err := p.Publish(e); err != nil {
		t.Fatal(err)
	}
	if err := f.run.tickAgent(a.ID, hookNotice{}, false); err != nil {
		t.Fatal(err)
	}
	got := f.agent(t, a.ID)
	if f.input.submits != 1 || got.LastDelivered != e.ID || got.Capacity.Fingerprint != "" {
		t.Fatalf("queued work did not take over: delivered=%s capacity=%+v submits=%d", got.LastDelivered, got.Capacity, f.input.submits)
	}
	if strings.Contains(f.input.pasted, "capacity-recovery") {
		t.Fatalf("queued correction replaced by continuation: %s", f.input.pasted)
	}
}

func TestCapacityRecoveryRejectsStaleFailureWithNewerSubmit(t *testing.T) {
	f, a := codexTurnErrorFixture(t, "new-turn", "old-turn", "server_overloaded", capacityError)
	f.input.screen = screenWithText("› previous task", "■ "+capacityError, "", "› ")
	for range 3 {
		if err := f.run.tickAgent(a.ID, hookNotice{}, false); err != nil {
			t.Fatal(err)
		}
	}
	if got := f.agent(t, a.ID); got.Capacity.Fingerprint != "" || f.input.submits != 0 {
		t.Fatalf("stale error recovered newer work: capacity=%+v submits=%d", got.Capacity, f.input.submits)
	}
}

func TestCapacityRecoverySuccessAfterDeadlineClearsEpisode(t *testing.T) {
	f, a := codexTurnErrorFixture(t, "t1", "t1", "server_overloaded", capacityError)
	p, _ := f.run.team.Agent(a.ID)
	now := f.cmd.now()
	f.run.cmd.clock = func() time.Time { return now }
	f.input.screen = screenWithText("■ "+capacityError, "", "› ")
	f.input.submit = func(prompt string) error {
		return p.WriteWitness(store.Witness{ID: "w2", At: now, SessionID: "s", TurnID: "retry", Transcript: a.Native.Transcript, Prompt: prompt})
	}
	if err := f.run.tickAgent(a.ID, hookNotice{}, false); err != nil {
		t.Fatal(err)
	}
	capacity := f.agent(t, a.ID).Capacity
	now = capacity.NextAt
	if err := f.run.tickAgent(a.ID, hookNotice{}, false); err != nil {
		t.Fatal(err)
	}
	now = capacity.Deadline.Add(time.Second)
	f.input.screen = screenWithText("• Done.", "", "› ")
	if err := f.run.tickAgent(a.ID, hookNotice{Kind: "turn-finished", SessionID: "s", TurnID: "retry", At: now}, false); err != nil {
		t.Fatal(err)
	}
	if got := f.agent(t, a.ID); got.Capacity.Fingerprint != "" || got.Activity != core.Idle || f.input.submits != 1 {
		t.Fatalf("success retained expired episode: activity=%s capacity=%+v submits=%d", got.Activity, got.Capacity, f.input.submits)
	}
}

func TestPendingCapacityContinuationDoesNotFollowNewTurn(t *testing.T) {
	f, a := codexTurnErrorFixture(t, "t1", "t1", "server_overloaded", capacityError)
	f.env["GANGLINE_HITCH_ID"] = string(a.ID)
	f.input.screen = screenWithText("■ "+capacityError, "", "› ")
	if err := f.run.tickAgent(a.ID, hookNotice{}, false); err != nil {
		t.Fatal(err)
	}
	a = f.agent(t, a.ID)
	p, _ := f.run.team.Agent(a.ID)
	// A published continuation can remain queued when a draft appears before drain.
	e := core.Envelope{ID: core.EnvelopeID("capacity-" + a.Capacity.Fingerprint), Token: "0123456789abcdef", Recipient: a.ID, To: a.Name, From: core.Sender{Kind: core.SenderGangline, Name: "capacity-recovery"}, Message: core.Message{Text: "Continue the interrupted work after the provider capacity error."}, CreatedAt: f.cmd.now()}
	if err := p.Publish(e); err != nil {
		t.Fatal(err)
	}
	f.setAgent(t, a, func(a *core.Agent) { a.Capacity.Submitted = true; a.Capacity.Attempts = 1 })
	now := f.cmd.now().Add(time.Second)
	f.run.cmd.clock = func() time.Time { return now }
	if err := p.WriteWitness(store.Witness{ID: "new", At: now, SessionID: "s", TurnID: "new", Transcript: a.Native.Transcript}); err != nil {
		t.Fatal(err)
	}
	f.input.screen = screenWithText("• Working (esc to interrupt)", "› ")
	if err := f.run.tickAgent(a.ID, hookNotice{}, false); err != nil {
		t.Fatal(err)
	}
	if f.input.submits != 0 {
		t.Fatalf("stale continuation submitted into newer running turn: submits=%d", f.input.submits)
	}
}

func TestLegacyClippedCapacityDoesNotBlockDueMail(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "codex")
	f.env["GANGLINE_HITCH_ID"] = string(a.ID)
	f.input.screen = screenWithText("■ "+capacityError, "", "› ")
	p, _ := f.run.team.Agent(a.ID)
	e := core.Envelope{ID: "queued", Token: "0123456789abcdef", Recipient: a.ID, To: a.Name, From: core.Sender{Kind: core.SenderSelfDeclared, Name: "operator"}, Message: core.Message{Text: "Proceed with correction."}, CreatedAt: f.cmd.now()}
	if err := p.Publish(e); err != nil {
		t.Fatal(err)
	}
	if err := f.run.tickAgent(a.ID, hookNotice{}, false); err != nil {
		t.Fatal(err)
	}
	if f.input.submits != 1 {
		t.Fatalf("queued correction not submitted: %d", f.input.submits)
	}
}

func TestPendingCapacityContinuationYieldsToDueCorrection(t *testing.T) {
	for _, busy := range []bool{false, true} {
		t.Run(fmt.Sprintf("busy=%v", busy), func(t *testing.T) {
			f, a := codexTurnErrorFixture(t, "t1", "t1", "server_overloaded", capacityError)
			f.env["GANGLINE_HITCH_ID"] = string(a.ID)
			f.input.screen = screenWithText("■ "+capacityError, "", "› ")
			if err := f.run.tickAgent(a.ID, hookNotice{}, false); err != nil {
				t.Fatal(err)
			}
			a = f.agent(t, a.ID)
			p, _ := f.run.team.Agent(a.ID)
			continuation := core.Envelope{ID: core.EnvelopeID("capacity-" + a.Capacity.Fingerprint), Token: "0123456789abcdef", Recipient: a.ID, To: a.Name, From: core.Sender{Kind: core.SenderGangline, Name: "capacity-recovery"}, Message: core.Message{Text: "Continue the interrupted work after the provider capacity error."}, CreatedAt: f.cmd.now()}
			correction := core.Envelope{ID: "correction", Token: "fedcba9876543210", Recipient: a.ID, To: a.Name, From: core.Sender{Kind: core.SenderSelfDeclared, Name: "operator"}, Message: core.Message{Text: "Use this correction."}, CreatedAt: f.cmd.now().Add(time.Second)}
			for _, e := range []core.Envelope{continuation, correction} {
				if err := p.Publish(e); err != nil {
					t.Fatal(err)
				}
			}
			f.setAgent(t, a, func(a *core.Agent) { a.Capacity.Submitted = true; a.Capacity.Attempts = 1 })
			if busy {
				f.input.screen = screenWithText("• Working (esc to interrupt)", "› ")
			}
			if _, err := f.run.drain(a.ID, correction.ID); err != nil {
				t.Fatal(err)
			}
			pending, err := p.ListNew()
			if err != nil || len(pending) != 0 || f.input.submits != 1 || f.agent(t, a.ID).LastDelivered != correction.ID {
				t.Fatalf("correction did not replace delayed continuation: pending=%+v err=%v submits=%d", pending, err, f.input.submits)
			}
		})
	}
}

func TestCapacityWitnessAdvancesDuringPaste(t *testing.T) {
	f, a := codexTurnErrorFixture(t, "t1", "t1", "server_overloaded", capacityError)
	f.env["GANGLINE_HITCH_ID"] = string(a.ID)
	f.input.screen = screenWithText("■ "+capacityError, "", "› ")
	if err := f.run.tickAgent(a.ID, hookNotice{}, false); err != nil {
		t.Fatal(err)
	}
	cap := f.agent(t, a.ID).Capacity
	now := cap.NextAt
	f.run.cmd.clock = func() time.Time { return now }
	p, _ := f.run.team.Agent(a.ID)
	calls := 0
	f.input.onForeground = func() {
		calls++
		if calls == 2 {
			if err := p.WriteWitness(store.Witness{ID: "new", At: now, SessionID: "s", TurnID: "new", Transcript: a.Native.Transcript}); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := f.run.tickAgent(a.ID, hookNotice{}, false); err != nil {
		t.Fatal(err)
	}
	if calls < 2 {
		t.Fatal("race callback never ran")
	}
	if f.input.submits != 0 {
		t.Fatalf("capacity continuation submitted after intervening native submit: %d", f.input.submits)
	}
}
