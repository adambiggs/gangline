package main

import (
	"bytes"
	"encoding/json"
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

// submitResumeHook runs the native submit hook for a resume note carrying token.
func submitResumeHook(t *testing.T, f *stateFixture, a core.Agent, token, text string) string {
	t.Helper()
	wire, err := envelopeText(core.Envelope{ID: "resume-c", Token: token, From: a.Compaction.ResumeFrom, Message: core.Message{Text: text}, Purpose: "resume"})
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(map[string]string{"hook_event_name": "UserPromptSubmit", "prompt": wire, "session_id": "s"})
	if err != nil {
		t.Fatal(err)
	}
	f.out.Reset()
	cmd := f.cmd
	cmd.stdin = bytes.NewReader(payload)
	if err := cmd.hook(nil); err != nil {
		t.Fatal(err)
	}
	return f.out.String()
}

// A blocked note never reaches the agent, so the note it was holding input
// for is gone: the compaction fails, which tells the agent its note was
// withheld, and a tick delivers held input at once. A blocked note from an
// earlier compaction says nothing about the current one.
func TestBlockedResumeNoteEndsHold(t *testing.T) {
	f, a := completedWithQueuedContinuation(t)
	a = f.setAgent(t, a, func(a *core.Agent) {
		a.Compaction.ResumeFrom = core.Sender{Kind: core.SenderSelfDeclared, Name: "compact"}
	})
	f.env["GANGLINE_HITCH_ID"] = string(a.ID)
	var ticked []string
	f.cmd.detach = func(id string, _ hookNotice) error { ticked = append(ticked, id); return nil }
	if out := submitResumeHook(t, f, a, "bbbbbbbbbbbbbbbb", "continue"); !strings.Contains(out, `"decision":"block"`) {
		t.Fatalf("other compaction's note was not blocked: %s", out)
	}
	if free, _ := inputFree(t, f, a); free {
		t.Fatal("another compaction's blocked note ended the hold")
	}
	ticked = nil
	if out := submitResumeHook(t, f, a, a.Compaction.ResumeToken, "continue, edited"); !strings.Contains(out, `"decision":"block"`) {
		t.Fatalf("altered note was not blocked: %s", out)
	}
	if free, reason := inputFree(t, f, a); !free {
		t.Fatalf("blocked note still holds input: %q", reason)
	}
	p, _ := f.run.team.Agent(a.ID)
	got, err := p.Read()
	if err != nil || got.Compaction.Status != "failed" {
		t.Fatalf("compaction after its note was blocked: %+v, %v", got.Compaction, err)
	}
	if _, err := p.ReadEnvelope("new", "failed-c"); err != nil {
		t.Fatalf("agent was not told its note was withheld: %v", err)
	}
	if len(ticked) != 1 || ticked[0] != string(a.ID) {
		t.Fatalf("blocked note started no tick: %v", ticked)
	}
	submitResumeHook(t, f, a, a.Compaction.ResumeToken, "continue")
	if got, err := p.Read(); err != nil || !strings.Contains(got.Compaction.Reason, "altered") {
		t.Fatalf("a failed compaction failed again: %+v, %v", got.Compaction, err)
	}
}

// A repeat of an admitted note is blocked, but the note itself already ran.
func TestRepeatedResumeNoteKeepsCompaction(t *testing.T) {
	f, a := completedWithQueuedContinuation(t)
	a = f.setAgent(t, a, func(a *core.Agent) {
		a.Compaction.ResumeFrom = core.Sender{Kind: core.SenderSelfDeclared, Name: "compact"}
		a.Compaction.ResumeAdmitted = true
	})
	f.env["GANGLINE_HITCH_ID"] = string(a.ID)
	f.cmd.detach = func(string, hookNotice) error { return nil }
	if out := submitResumeHook(t, f, a, a.Compaction.ResumeToken, "continue"); !strings.Contains(out, "already admitted") {
		t.Fatalf("repeated note: %s", out)
	}
	p, _ := f.run.team.Agent(a.ID)
	if got, err := p.Read(); err != nil || got.Compaction.Status != "completed" {
		t.Fatalf("repeated note failed an admitted compaction: %+v, %v", got.Compaction, err)
	}
}

// A note blocked before completion was confirmed says the compaction may
// have run, not that it did.
func TestBlockedResumeNoteBeforeCompletion(t *testing.T) {
	f, a := completedWithQueuedContinuation(t)
	a = f.setAgent(t, a, func(a *core.Agent) {
		a.Compaction.ResumeFrom = core.Sender{Kind: core.SenderSelfDeclared, Name: "compact"}
		a.Compaction.Status, a.Compaction.CompletedAt = "submitted", time.Time{}
	})
	f.env["GANGLINE_HITCH_ID"] = string(a.ID)
	f.cmd.detach = func(string, hookNotice) error { return nil }
	submitResumeHook(t, f, a, a.Compaction.ResumeToken, "continue, edited")
	p, _ := f.run.team.Agent(a.ID)
	e, err := p.ReadEnvelope("new", "failed-c")
	if err != nil || !strings.Contains(e.Message.Text, "may have run") {
		t.Fatalf("notice for a note blocked before completion: %+v, %v", e.Message, err)
	}
}

// A compaction with no resume token yet has no note in the harness, so a
// typed resume header says nothing about it.
func TestTypedResumeHeaderKeepsQueuedCompaction(t *testing.T) {
	f, a, _ := claudeRecipient(t)
	now := f.cmd.now()
	a = f.setAgent(t, a, func(a *core.Agent) {
		a.Compaction = &core.Compaction{ID: "c", Resume: core.Message{Text: "continue"}, StartedAt: now, Deadline: now.Add(operationTimeout), Status: "queued"}
	})
	f.env["GANGLINE_HITCH_ID"] = string(a.ID)
	f.cmd.detach = func(string, hookNotice) error { return nil }
	payload, err := json.Marshal(map[string]string{"hook_event_name": "UserPromptSubmit", "prompt": "[gang:x# resume] continue", "session_id": "s"})
	if err != nil {
		t.Fatal(err)
	}
	f.cmd.stdin = bytes.NewReader(payload)
	if err := f.cmd.hook(nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(f.out.String(), `"decision":"block"`) {
		t.Fatalf("typed resume header was not blocked: %s", f.out)
	}
	if got := f.agent(t, a.ID); got.Compaction.Status != "queued" {
		t.Fatalf("typed resume header failed a queued compaction: %+v", got.Compaction)
	}
}
