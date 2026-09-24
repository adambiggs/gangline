package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
)

func nativeFailureFixture(t *testing.T) (*stateFixture, core.Agent, func(map[string]string) hookNotice) {
	t.Helper()
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "claude-code")
	f.env["GANGLINE_HITCH_ID"] = string(a.ID)
	f.input.screen = screenWithText("────────────────────────────────────────────────────────────────────────────────", "❯", "────────────────────────────────────────────────────────────────────────────────")
	now := f.cmd.now()
	f.run.cmd.clock = func() time.Time { return now }
	hook := f.cmd
	hook.clock = func() time.Time { return now }
	var notice hookNotice
	hook.detach = func(_ string, n hookNotice) error { notice = n; return nil }
	call := func(body map[string]string) hookNotice {
		t.Helper()
		now = now.Add(time.Second)
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		hook.stdin = bytes.NewReader(data)
		notice = hookNotice{}
		if err := hook.handleHook(nil); err != nil {
			t.Fatal(err)
		}
		return notice
	}
	return f, a, call
}

func TestFailedNativeStartupTurnVisibleUntilNewTurn(t *testing.T) {
	f, a, call := nativeFailureFixture(t)
	call(map[string]string{"hook_event_name": "UserPromptSubmit", "session_id": "s", "prompt_id": "startup", "prompt": "[gang:lead#startup-token assignment] contract"})
	n := call(map[string]string{"hook_event_name": "StopFailure", "session_id": "s", "prompt_id": "startup", "error": "Login expired", "error_details": "Please run /login"})
	if n.Kind != "turn-failed" || n.TurnID != "startup" {
		t.Fatalf("notice: %+v", n)
	}
	if err := f.run.tickAgent(a.ID, n, true); err != nil {
		t.Fatal(err)
	}
	p, _ := f.run.team.Agent(a.ID)
	got, err := p.Read()
	if err != nil {
		t.Fatal(err)
	}
	if got.Activity == core.Idle || got.Native.FailedTurn != "startup" || !strings.Contains(got.Evidence, "Login expired: Please run /login") {
		t.Fatalf("failed startup: activity=%s evidence=%q native=%+v", got.Activity, got.Evidence, got.Native)
	}
	f.out.Reset()
	if err := f.cmd.status([]string{"worker", "--why"}); err != nil || !strings.Contains(f.out.String(), "Login expired") {
		t.Fatalf("status --why: %q, %v", f.out.String(), err)
	}
	f.out.Reset()
	if err := f.cmd.roster(nil); err != nil || strings.Contains(f.out.String(), " idle ") {
		t.Fatalf("roster: %q, %v", f.out.String(), err)
	}
	if err := f.cmd.wait([]string{"worker", "--timeout", "0"}); err == nil {
		t.Fatal("wait accepted failed turn")
	}
	audit, err := os.ReadFile(f.run.team.Log)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(audit, []byte(`"status":"turn-failed"`)) || !bytes.Contains(audit, []byte("Login expired")) {
		t.Fatalf("audit lacks failure and reason: %s", audit)
	}
	call(map[string]string{"hook_event_name": "UserPromptSubmit", "session_id": "s", "prompt_id": "retry", "prompt": "retry"})
	if err := f.run.tickAgent(a.ID, hookNotice{Kind: "turn-started", TurnID: "retry", SessionID: "s"}, true); err != nil {
		t.Fatal(err)
	}
	got, err = p.Read()
	if err != nil {
		t.Fatal(err)
	}
	if got.Native.TurnFailure != "" {
		t.Fatalf("new turn kept old failure: %+v", got.Native)
	}
}

func TestLateNativeFailureCannotReplaceNewTurn(t *testing.T) {
	f, a, call := nativeFailureFixture(t)
	call(map[string]string{"hook_event_name": "UserPromptSubmit", "session_id": "s", "prompt_id": "old", "prompt": "old"})
	old := call(map[string]string{"hook_event_name": "StopFailure", "session_id": "s", "prompt_id": "old", "error": "old error"})
	call(map[string]string{"hook_event_name": "UserPromptSubmit", "session_id": "s", "prompt_id": "new", "prompt": "new"})
	f.input.screen = screenWithText("• Working (esc to interrupt)", "────────────────────────────────────────────────────────────────────────────────", "❯", "────────────────────────────────────────────────────────────────────────────────")
	if err := f.run.tickAgent(a.ID, old, true); err != nil {
		t.Fatal(err)
	}
	p, _ := f.run.team.Agent(a.ID)
	got, _ := p.Read()
	if got.Native.TurnFailure != "" || got.Activity != core.Busy {
		t.Fatalf("delayed tick assigned old failure: activity=%s native=%+v", got.Activity, got.Native)
	}
	late := call(map[string]string{"hook_event_name": "StopFailure", "session_id": "s", "prompt_id": "old", "error": "old error"})
	if err := f.run.tickAgent(a.ID, late, true); err != nil {
		t.Fatal(err)
	}
	got, _ = p.Read()
	if got.Native.TurnFailure != "" || got.Activity != core.Busy {
		t.Fatalf("late callback assigned old failure: activity=%s native=%+v", got.Activity, got.Native)
	}
}

func TestSubmitDuringFailureTickReconcilesAfterSave(t *testing.T) {
	f, a, call := nativeFailureFixture(t)
	call(map[string]string{"hook_event_name": "UserPromptSubmit", "session_id": "s", "prompt_id": "old", "prompt": "old"})
	failure := call(map[string]string{"hook_event_name": "StopFailure", "session_id": "s", "prompt_id": "old", "error": "old error"})
	var newStart hookNotice
	f.run.afterWitnessRead = func() {
		f.run.afterWitnessRead = nil
		// The submit hook reads state before the failure tick saves its decision.
		newStart = call(map[string]string{"hook_event_name": "UserPromptSubmit", "session_id": "s", "prompt_id": "new", "prompt": "new"})
	}
	f.input.screen = screenWithText("• Working (esc to interrupt)", "────────────────────────────────────────────────────────────────────────────────", "❯", "────────────────────────────────────────────────────────────────────────────────")
	if err := f.run.tickAgent(a.ID, failure, true); err != nil {
		t.Fatal(err)
	}
	if newStart.Kind != "turn-started" || newStart.TurnID != "new" {
		t.Fatalf("concurrent submit did not schedule reconciliation: %+v", newStart)
	}
	p, _ := f.run.team.Agent(a.ID)
	got, _ := p.Read()
	if got.Native.FailedTurn != "old" {
		t.Fatalf("failure tick did not finish after submit: %+v", got.Native)
	}
	if err := f.run.tickAgent(a.ID, newStart, true); err != nil {
		t.Fatal(err)
	}
	got, _ = p.Read()
	if got.Native.TurnFailure != "" || got.Activity != core.Busy {
		t.Fatalf("new turn retained old failure: activity=%s native=%+v", got.Activity, got.Native)
	}
}

func TestNativeFailureSurvivesProbeError(t *testing.T) {
	f, a, call := nativeFailureFixture(t)
	call(map[string]string{"hook_event_name": "UserPromptSubmit", "session_id": "s", "prompt_id": "turn", "prompt": "prompt"})
	failure := call(map[string]string{"hook_event_name": "StopFailure", "session_id": "s", "prompt_id": "turn", "error": "Login expired"})
	probe := &failingActivityProbe{f.input, context.DeadlineExceeded}
	f.run.cmd.inputBackend = probe
	if err := f.run.tickAgent(a.ID, failure, true); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("probe error = %v", err)
	}
	p, _ := f.run.team.Agent(a.ID)
	got, _ := p.Read()
	if got.Activity != core.Unknown || !strings.Contains(got.Evidence, "Login expired") {
		t.Fatalf("probe error hid native failure: %+v", got)
	}
	probe.err = nil
	if err := f.run.tickAgent(a.ID, hookNotice{}, true); err != nil {
		t.Fatal(err)
	}
	got, _ = p.Read()
	if got.Native.FailedTurn != "turn" || got.Activity == core.Idle || !strings.Contains(got.Evidence, "Login expired") {
		t.Fatalf("failure lost after probe recovered: %+v", got)
	}
}

func TestStaleSuccessDoesNotClearNewFailure(t *testing.T) {
	f, a, call := nativeFailureFixture(t)
	call(map[string]string{"hook_event_name": "UserPromptSubmit", "session_id": "s", "prompt_id": "old", "prompt": "old"})
	call(map[string]string{"hook_event_name": "UserPromptSubmit", "session_id": "s", "prompt_id": "new", "prompt": "new"})
	failure := call(map[string]string{"hook_event_name": "StopFailure", "session_id": "s", "prompt_id": "new", "error": "new error"})
	if err := f.run.tickAgent(a.ID, failure, true); err != nil {
		t.Fatal(err)
	}
	oldSuccess := call(map[string]string{"hook_event_name": "Stop", "session_id": "s", "prompt_id": "old"})
	if err := f.run.tickAgent(a.ID, oldSuccess, true); err != nil {
		t.Fatal(err)
	}
	p, _ := f.run.team.Agent(a.ID)
	got, _ := p.Read()
	if got.Native.FailedTurn != "new" || !strings.Contains(got.Evidence, "new error") {
		t.Fatalf("stale success cleared failure: %+v", got)
	}
}

func TestUnidentifiedFailureStaysUnknownUntilIdentifiedSuccess(t *testing.T) {
	f, a, call := nativeFailureFixture(t)
	call(map[string]string{"hook_event_name": "UserPromptSubmit", "session_id": "s", "prompt_id": "first", "prompt": "first"})
	failure := call(map[string]string{"hook_event_name": "StopFailure", "session_id": "s", "error": "Login expired"})
	if err := f.run.tickAgent(a.ID, failure, true); err != nil {
		t.Fatal(err)
	}
	p, _ := f.run.team.Agent(a.ID)
	got, _ := p.Read()
	if got.Activity != core.Unknown || !strings.Contains(got.Evidence, "without turn identity") {
		t.Fatalf("missing ID looks healthy: %+v", got)
	}
	call(map[string]string{"hook_event_name": "UserPromptSubmit", "session_id": "s", "prompt_id": "second", "prompt": "second"})
	if err := f.run.tickAgent(a.ID, hookNotice{Kind: "turn-started", TurnID: "second", SessionID: "s"}, true); err != nil {
		t.Fatal(err)
	}
	got, _ = p.Read()
	if got.Activity != core.Unknown {
		t.Fatalf("uncertainty cleared before success: %+v", got)
	}
	success := call(map[string]string{"hook_event_name": "Stop", "session_id": "s", "prompt_id": "second"})
	if err := f.run.tickAgent(a.ID, success, true); err != nil {
		t.Fatal(err)
	}
	got, _ = p.Read()
	if got.Native.TurnFailure != "" || got.Activity != core.Idle {
		t.Fatalf("success did not clear uncertainty: %+v", got)
	}
}

func TestLongNativeFailureReasonStillChangesState(t *testing.T) {
	f, a, call := nativeFailureFixture(t)
	call(map[string]string{"hook_event_name": "UserPromptSubmit", "session_id": "s", "prompt_id": "first", "prompt": "first"})
	failure := call(map[string]string{"hook_event_name": "StopFailure", "session_id": "s", "prompt_id": "first", "error": "authentication_failed", "error_details": strings.Repeat("x", 5000)})
	if len(failure.Failure) > 4096 || !strings.Contains(failure.Failure, "[truncated]") {
		t.Fatalf("unbounded failure: %d bytes", len(failure.Failure))
	}
	if err := f.run.tickAgent(a.ID, failure, true); err != nil {
		t.Fatal(err)
	}
	p, _ := f.run.team.Agent(a.ID)
	got, _ := p.Read()
	if got.Activity == core.Idle || !strings.Contains(got.Evidence, "authentication_failed") {
		t.Fatalf("long failure was dropped: %+v", got)
	}
}
