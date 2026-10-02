package main

import (
	"strings"
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
)

// completedWithQueuedContinuation is a Claude agent whose compaction has just
// completed while its resume note still waits in the harness's own queue.
func completedWithQueuedContinuation(t *testing.T) (*stateFixture, core.Agent) {
	t.Helper()
	f, a, _ := claudeRecipient(t)
	now := f.cmd.now()
	a = f.setAgent(t, a, func(a *core.Agent) {
		a.Compaction = &core.Compaction{
			ID: "c", Resume: core.Message{Text: "continue"}, ResumeToken: "aaaaaaaaaaaaaaaa",
			StartedAt: now.Add(-time.Minute), Deadline: now.Add(-30 * time.Second),
			CompletedAt: now, Status: "completed", Continuation: true,
		}
	})
	return f, a
}

func inputFree(t *testing.T, f *stateFixture, a core.Agent) (bool, string) {
	t.Helper()
	l, got, err := f.run.acquire(a.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	defer f.run.release(l)
	c, err := loadCollar(got.Collar, f.run.settings)
	if err != nil {
		t.Fatal(err)
	}
	b, err := f.run.input()
	if err != nil {
		t.Fatal(err)
	}
	v, err := f.run.inputState(l, &got, b, c)
	if err != nil {
		t.Fatal(err)
	}
	return v.Free, v.Reason
}

// The queued note's submit hook takes the agent lock. A message typed ahead
// of it would hold that lock waiting for its own hook, which the harness runs
// only after the note's, so input waits for the note to be admitted.
func TestInputWaitsForQueuedContinuationAfterCompaction(t *testing.T) {
	f, a := completedWithQueuedContinuation(t)
	if err := f.cmd.send([]string{"worker", "--from", "operator", "after compaction"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(f.out.String(), "queued") || f.input.pasted != "" || f.input.submits != 0 {
		t.Fatalf("send ahead of the queued note: output=%q pasted=%q submits=%d", f.out, f.input.pasted, f.input.submits)
	}
	if free, reason := inputFree(t, f, a); free || !strings.Contains(reason, "continuation") {
		t.Fatalf("held input: free=%v reason=%q", free, reason)
	}
	a = f.setAgent(t, a, func(a *core.Agent) { a.Compaction.ResumeAdmitted = true })
	if free, reason := inputFree(t, f, a); !free {
		t.Fatalf("admitted note still holds input: %q", reason)
	}
}

// A note whose hook never runs holds input no longer than one operation.
func TestQueuedContinuationHoldEnds(t *testing.T) {
	f, a := completedWithQueuedContinuation(t)
	completed := a.Compaction.CompletedAt
	f.cmd.clock = func() time.Time { return completed.Add(operationTimeout - time.Millisecond) }
	f.run.cmd = f.cmd
	if free, _ := inputFree(t, f, a); free {
		t.Fatal("hold ended early")
	}
	f.cmd.clock = func() time.Time { return completed.Add(operationTimeout) }
	f.run.cmd = f.cmd
	if free, reason := inputFree(t, f, a); !free {
		t.Fatalf("hold outlived its bound: %q", reason)
	}
}

// Only a note the harness still holds after a completed compaction waits.
func TestNoQueuedContinuationLeavesInputFree(t *testing.T) {
	for name, change := range map[string]func(*core.Compaction){
		"compaction failed": func(c *core.Compaction) { c.Status = "failed" },
	} {
		f, a := completedWithQueuedContinuation(t)
		a = f.setAgent(t, a, func(a *core.Agent) { change(a.Compaction) })
		if free, reason := inputFree(t, f, a); !free {
			t.Errorf("%s: input held: %q", name, reason)
		}
	}
}
