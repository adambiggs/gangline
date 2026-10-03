package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
)

func assertUnlockFailure(t *testing.T, f *stateFixture, a core.Agent, err, operationErr, unlockErr error) {
	t.Helper()
	if !errors.Is(err, unlockErr) || (operationErr != nil && !errors.Is(err, operationErr)) {
		t.Fatalf("operation/unlock errors not retained: %v", err)
	}
	if got := f.run.tickFailed("command", a.ID, err); !errors.Is(got, unlockErr) {
		t.Fatalf("failure recording lost unlock error: %v", got)
	}
	failures := tickFailures(t, f)
	if len(failures) != 1 || !strings.Contains(failures[0].Reason, unlockErr.Error()) || (operationErr != nil && !strings.Contains(failures[0].Reason, operationErr.Error())) {
		t.Fatalf("failure diagnostic: %+v", failures)
	}
	p, _ := f.run.team.Agent(a.ID)
	l, lockErr := p.TryLock()
	if lockErr != nil {
		t.Fatalf("agent lock retained: %v", lockErr)
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	if f.input.pasted != "" || f.input.submits != 0 || len(f.input.keys) != 0 {
		t.Fatal("unlock failure caused input")
	}
}

func TestTickPreservesUnlockFailureWithObservationError(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "codex")
	f.input.screen = screenWithText("no input surface")
	unlockErr := errors.New("detached tick launch failed")
	calls := 0
	f.run.cmd.detach = func(string, hookNotice) error { calls++; return unlockErr }
	f.run.wake = []core.HitchID{"other"}
	err := f.run.tickAgent(a.ID, hookNotice{}, false)
	assertUnlockFailure(t, f, a, err, harness.ErrNoComposer, unlockErr)
	if calls != 1 {
		t.Fatalf("detach called %d times", calls)
	}
}

func TestTickPreservesUnlockFailureWithoutOperationError(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "codex")
	p, _ := f.run.team.Agent(a.ID)
	l, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	a.Status = core.Failed
	a.Pane = ""
	if err := l.Save(a); err != nil {
		t.Fatal(err)
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	unlockErr := errors.New("detached tick launch failed")
	calls := 0
	f.run.cmd.detach = func(string, hookNotice) error { calls++; return unlockErr }
	f.run.wake = []core.HitchID{"other"}
	err = f.run.tickAgent(a.ID, hookNotice{}, false)
	assertUnlockFailure(t, f, a, err, nil, unlockErr)
	if calls != 1 {
		t.Fatalf("detach called %d times", calls)
	}
}

func TestDrainPreservesUnlockFailureWithOperationError(t *testing.T) {
	f, a, p := pendingFixture(t, "codex")
	operationErr := errors.New("capture I/O failure")
	f.input.captureErr = operationErr
	unlockErr := errors.New("detached tick launch failed")
	calls := 0
	f.run.cmd.detach = func(string, hookNotice) error { calls++; return unlockErr }
	f.run.wake = []core.HitchID{"other"}
	l, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	result, err := f.run.drainFrom(l, a, "")
	if result != "queued" {
		t.Fatalf("drain result: %q", result)
	}
	assertUnlockFailure(t, f, a, err, operationErr, unlockErr)
	if calls != 1 {
		t.Fatalf("detach called %d times", calls)
	}
	if q := inboxNew(t, f, a); len(q) != 1 || q[0].ID != "pending" {
		t.Fatalf("queue changed: %+v", q)
	}
}
