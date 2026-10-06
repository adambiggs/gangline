package main

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/adambiggs/gangline/store"
)

func TestHookStatusRegressionRetainedOldSession(t *testing.T) {
	f, a, _ := movedSessionFixture(t, "n", `{"type":"user","sessionId":"s","promptId":"p1"}`, `{"type":"continued-in","sessionId":"s","continuedInSessionId":"n"}`)
	p, _ := f.run.team.Agent(a.ID)
	if err := p.WriteStatusHook(store.StatusHook{Kind: "activity", SessionID: "s", At: f.cmd.now()}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := f.run.tickAgent(a.ID, hookNotice{}, false); err != nil {
			t.Errorf("sweep %d: %v", i, err)
		}
	}
}

func TestHookStatusRegressionRosterDefersCapacity(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "codex")
	p, _ := f.run.team.Agent(a.ID)
	now := f.cmd.now()
	if err := p.WriteStatusHook(store.StatusHook{Kind: "activity", At: now}); err != nil {
		t.Fatal(err)
	}
	f.run.cmd.clock = func() time.Time { return now }
	f.input.screen = screenWithText("› solve the task", "■ Selected model is at capacity. Please try a different model.", "", "› ")
	for i := 0; i < 3; i++ {
		now = now.Add(statusHookFreshness)
		l, current, err := f.run.acquire(a.ID, false)
		if err != nil {
			t.Fatal(err)
		}
		c, err := loadCollar(current.Collar, f.run.settings)
		if err != nil {
			t.Fatal(err)
		}
		err = f.run.observeRosterStatus(l, &current, c)
		if closeErr := f.run.unlock(l); closeErr != nil {
			t.Fatal(closeErr)
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := f.run.tickAgent(a.ID, hookNotice{}, false); err != nil {
		t.Fatal(err)
	}
	if got := f.agent(t, a.ID); got.Capacity.Fingerprint == "" {
		t.Fatalf("capacity never discovered after three roster+watchdog cycles; captures=%d", f.input.captures)
	}
}

func TestHookStatusRegressionLateOldFailureOverwritesCurrent(t *testing.T) {
	f, a, call := nativeFailureFixture(t)
	call(map[string]string{"hook_event_name": "UserPromptSubmit", "session_id": "s", "prompt_id": "new", "prompt": "work"})
	current := call(map[string]string{"hook_event_name": "StopFailure", "session_id": "s", "prompt_id": "new", "error": "Login expired"})
	call(map[string]string{"hook_event_name": "StopFailure", "session_id": "s", "prompt_id": "old", "error": "old failure"})
	if err := f.run.tickAgent(a.ID, current, true); err != nil {
		t.Fatal(err)
	}
	if got := f.agent(t, a.ID); got.Native.FailedTurn != "new" {
		t.Fatalf("current failure lost although its detached notice runs: failure=%q failedTurn=%q activity=%s", got.Native.TurnFailure, got.Native.FailedTurn, got.Activity)
	}
}

func TestHookStatusRosterDoesNotAcknowledgeFailure(t *testing.T) {
	f, a, call := nativeFailureFixture(t)
	call(map[string]string{"hook_event_name": "UserPromptSubmit", "session_id": "s", "prompt_id": "new", "prompt": "work"})
	call(map[string]string{"hook_event_name": "StopFailure", "session_id": "s", "prompt_id": "new", "error": "Login expired"})
	l, current, err := f.run.acquire(a.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	c, err := loadCollar(a.Collar, f.run.settings)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.run.observeRosterStatus(l, &current, c); err != nil {
		t.Fatal(err)
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	if current.Native.HookSequence != 0 {
		t.Fatalf("roster consumed sequence %d", current.Native.HookSequence)
	}
	call(map[string]string{"hook_event_name": "StopFailure", "session_id": "s", "prompt_id": "old", "error": "old failure"})
	if err := f.run.tickAgent(a.ID, hookNotice{}, false); err != nil {
		t.Fatal(err)
	}
	got := f.agent(t, a.ID)
	if got.Native.FailedTurn != "new" || got.Native.HookSequence == 0 {
		t.Fatalf("failure/ack lost: %+v", got.Native)
	}
}

func TestHookStatusLateQueuedFailureCannotReplaceNewerFailure(t *testing.T) {
	f, a, call := nativeFailureFixture(t)
	transcript := filepath.Join(t.TempDir(), "turns.jsonl")
	appendLines(t, transcript,
		`{"type":"user","promptId":"p1","timestamp":"2026-10-02T09:00:00Z"}`,
		`{"type":"user","promptId":"p2","timestamp":"2026-10-02T09:01:00Z"}`,
		`{"type":"user","promptId":"p3","timestamp":"2026-10-02T09:02:00Z"}`)
	call(map[string]string{"hook_event_name": "UserPromptSubmit", "session_id": "s", "prompt_id": "p1", "prompt": "work", "transcript_path": transcript})
	call(map[string]string{"hook_event_name": "StopFailure", "session_id": "s", "prompt_id": "p3", "error": "current failure", "transcript_path": transcript})
	call(map[string]string{"hook_event_name": "StopFailure", "session_id": "s", "prompt_id": "p2", "error": "old failure", "transcript_path": transcript})
	if err := f.run.tickAgent(a.ID, hookNotice{}, false); err != nil {
		t.Fatal(err)
	}
	if got := f.agent(t, a.ID); got.Native.FailedTurn != "p3" {
		t.Fatalf("failure regressed: %+v", got.Native)
	}
}
