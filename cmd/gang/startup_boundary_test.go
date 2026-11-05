//go:build acceptance

package main

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/store"
)

func TestStartupFailureProbeError(t *testing.T) {
	f, a := codexTurnErrorFixture(t, "t1", "t1", "model_not_found", "unsupported model")
	lead := f.addHitched(t, "l", "lead", "lead", "")
	a = f.setAgent(t, a, func(a *core.Agent) { a.HitchedBy = lead.ID })
	a, wire := deliveredStartup(t, f, a)
	p, _ := f.run.team.Agent(a.ID)
	if err := p.WriteWitness(store.Witness{ID: "startup-witness", At: a.CreatedAt, SessionID: "s", TurnID: "t1", Prompt: wire, Transcript: a.Native.Transcript}); err != nil {
		t.Fatal(err)
	}
	f.input.captureErr = errors.New("synthetic pane probe failure")
	if err := f.run.tickAgent(a.ID, hookNotice{}, false); err == nil {
		t.Fatal("expected probe error")
	}
	got := f.agent(t, a.ID)
	lp, _ := f.run.team.Agent(lead.ID)
	notices, err := lp.ListNew()
	if err != nil || len(notices) != 1 {
		t.Fatalf("notices=%v err=%v", len(notices), err)
	}
	if err := f.cmd.status([]string{"worker"}); err != nil {
		t.Fatal(err)
	}
	t.Logf("plain status=%q; native failure=%q; notice count=%d", f.out.String(), got.Native.TurnFailure, len(notices))
	if got.Activity != core.Blocked {
		t.Fatalf("authoritative failed startup became %s after an unrelated pane probe failure", got.Activity)
	}
}

func TestRecognizedPromptChangesAndResolves(t *testing.T) {
	f := newStateFixture(t)
	lead := f.addHitched(t, "l", "lead", "lead", "")
	a := f.addHitched(t, "a", "worker", "", lead.ID)
	f.setAgent(t, a, func(a *core.Agent) { a.Status, a.Activity = core.Booting, core.Unknown })
	f.input.screen = screenWithText("Trust this folder?", "› 1. Trust and continue")
	now := f.cmd.now()
	f.cmd.clock = func() time.Time { return now }
	if err := f.cmd.tick([]string{"--agent", "worker"}); err != nil {
		t.Fatal(err)
	}
	lp, _ := f.run.team.Agent(lead.ID)
	notices, err := lp.ListNew()
	if err != nil || len(notices) != 1 {
		t.Fatalf("notices=%+v err=%v", notices, err)
	}
	// A different recognized blocked condition should send a fresh notice.
	f.input.screen = screenWithText("Giving this request a little extra thought", "› 1. Retry with a faster model", "  2. Keep waiting")
	now = now.Add(time.Second)
	if err := f.cmd.tick([]string{"--agent", "worker"}); err != nil {
		t.Fatal(err)
	}
	notices, err = lp.ListNew()
	if err != nil || len(notices) != 2 {
		t.Fatalf("changed prompt notices=%+v err=%v", notices, err)
	}
	f.input.screen = screenWithText("READY", "› ")
	now = now.Add(time.Second)
	if err := f.cmd.tick([]string{"--agent", "worker"}); err != nil {
		t.Fatal(err)
	}
	got := f.agent(t, a.ID)
	if got.Status != core.Active {
		t.Fatalf("resolved prompt status=%s evidence=%q", got.Status, got.Evidence)
	}
	now = now.Add(2 * startupPromptReminder)
	if err := f.cmd.tick([]string{"--agent", "worker"}); err != nil {
		t.Fatal(err)
	}
	notices, err = lp.ListNew()
	if err != nil || len(notices) != 2 {
		t.Fatalf("resolved prompt notices=%+v err=%v", notices, err)
	}
}

func TestHookStartupFailureNotifiesHitcher(t *testing.T) {
	f, a, call := nativeFailureFixture(t)
	lead := f.addHitched(t, "l", "lead", "lead", "")
	a = f.setAgent(t, a, func(a *core.Agent) { a.HitchedBy = lead.ID })
	_, wire := deliveredStartup(t, f, a, "startup")
	call(map[string]string{"hook_event_name": "UserPromptSubmit", "session_id": "s", "prompt_id": "startup", "prompt": wire})
	n := call(map[string]string{"hook_event_name": "StopFailure", "session_id": "s", "prompt_id": "startup", "error": "Login expired", "error_details": "Please run /login"})
	if err := f.run.tickAgent(a.ID, n, true); err != nil {
		t.Fatal(err)
	}
	got := f.agent(t, a.ID)
	lp, _ := f.run.team.Agent(lead.ID)
	notices, err := lp.ListNew()
	if got.Activity != core.Blocked || err != nil || len(notices) != 1 || !strings.Contains(notices[0].Message.Text, "Login expired") {
		t.Fatalf("activity=%s notices=%+v err=%v", got.Activity, notices, err)
	}
}

func TestStartupFailureAfterQueuedPrompt(t *testing.T) {
	f, a, call := nativeFailureFixture(t)
	lead := f.addHitched(t, "l", "lead", "lead", "")
	a = f.setAgent(t, a, func(a *core.Agent) { a.HitchedBy = lead.ID })
	_, wire := deliveredStartup(t, f, a, "startup")
	call(map[string]string{"hook_event_name": "UserPromptSubmit", "session_id": "s", "prompt_id": "startup", "prompt": wire})
	// Claude reports input queued during a running turn with that running turn's ID.
	call(map[string]string{"hook_event_name": "UserPromptSubmit", "session_id": "s", "prompt_id": "startup", "prompt": "additional instructions queued behind startup"})
	n := call(map[string]string{"hook_event_name": "StopFailure", "session_id": "s", "prompt_id": "startup", "error": "Login expired", "error_details": "Please run /login"})
	if err := f.run.tickAgent(a.ID, n, true); err != nil {
		t.Fatal(err)
	}
	got := f.agent(t, a.ID)
	lp, _ := f.run.team.Agent(lead.ID)
	notices, err := lp.ListNew()
	t.Logf("activity=%s failed_turn=%q startup_receipt=%q notice_count=%d", got.Activity, got.Native.FailedTurn, got.LastDelivered, len(notices))
	if err != nil || len(notices) != 1 {
		t.Fatalf("startup failure lost after queued prompt: notices=%+v err=%v", notices, err)
	}
}
