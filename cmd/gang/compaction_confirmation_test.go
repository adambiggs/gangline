package main

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/store"
	"github.com/adambiggs/gangline/substrate"
)

func compactionFixture(t *testing.T) (*stateFixture, core.Agent, store.AgentPaths) {
	t.Helper()
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "codex")
	p, _ := f.run.team.Agent(a.ID)
	a.Native.SessionID = "s"
	l, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Save(a); err != nil {
		t.Fatal(err)
	}
	l.Close()
	f.input.submit = func(prompt string) error {
		if strings.HasPrefix(prompt, "/compact") {
			return nil
		}
		return p.WriteWitness(store.Witness{ID: "resume", At: f.cmd.now(), SessionID: "s", Prompt: prompt})
	}
	return f, a, p
}

func TestCompactionObservesIdleBeforeStarting(t *testing.T) {
	f, a, p := compactionFixture(t)
	f.input.screen = screenWithText("Working (esc to interrupt)", "› ")
	if err := f.cmd.compact([]string{"worker", "--resume", "resume only after compaction"}); err != nil {
		t.Fatal(err)
	}
	got, err := p.Read()
	if err != nil {
		t.Fatal(err)
	}
	if f.input.submits != 0 || got.Compaction.Status != "queued" || !strings.Contains(f.out.String(), "waiting for native idle") {
		t.Fatalf("busy compaction: submits=%d status=%s output=%s", f.input.submits, got.Compaction.Status, f.out)
	}
	f.input.screen = screenWithText("READY", "› ")
	if err := f.run.tickAgent(a.ID, hookNotice{Kind: "turn-finished", SessionID: "s", At: f.cmd.now()}, false); err != nil {
		t.Fatal(err)
	}
	got, err = p.Read()
	if err != nil {
		t.Fatal(err)
	}
	if got.Compaction.Status != "submitted" || f.input.submits != 1 || got.Compaction.Continuation {
		t.Fatalf("idle seam: %+v submits=%d", got.Compaction, f.input.submits)
	}
}

func TestCompactionRequiresFreshSameSessionCompletion(t *testing.T) {
	f, a, p := compactionFixture(t)
	err := f.cmd.compact([]string{"worker", "--resume", "resume only after compaction"})
	var ce commandError
	if !errors.As(err, &ce) || ce.status != exitUnknown || !strings.Contains(err.Error(), "unconfirmed") {
		t.Fatalf("submission claimed completion: %v", err)
	}
	got, err := p.Read()
	if err != nil {
		t.Fatal(err)
	}
	if f.input.submits != 1 || got.Compaction.Continuation {
		t.Fatalf("premature continuation: %+v submits=%d", got.Compaction, f.input.submits)
	}
	start := got.Compaction.StartedAt
	checkpoint := start.Add(time.Second)
	if err := f.run.tickAgent(a.ID, hookNotice{SessionID: "s", Readings: []core.Reading{{Kind: "compaction-checkpoint", At: &checkpoint}}}, false); err != nil {
		t.Fatal(err)
	}
	if f.input.submits != 1 {
		t.Fatal("checkpoint resumed without completion")
	}
	if err := f.run.tickAgent(a.ID, hookNotice{Kind: "compaction-finished", SessionID: "s", At: start.Add(-time.Second)}, false); err != nil {
		t.Fatal(err)
	}
	if f.input.submits != 1 {
		t.Fatal("stale completion resumed")
	}
	if err := f.run.tickAgent(a.ID, hookNotice{Kind: "compaction-finished", SessionID: "other", At: start.Add(time.Second)}, false); err == nil {
		t.Fatal("foreign completion accepted")
	}
	if f.input.submits != 1 {
		t.Fatal("foreign completion resumed")
	}
	for range 2 {
		if err := f.run.tickAgent(a.ID, hookNotice{Kind: "compaction-finished", SessionID: "s", At: start.Add(time.Second)}, false); err != nil {
			t.Fatal(err)
		}
	}
	got, err = p.Read()
	if err != nil {
		t.Fatal(err)
	}
	if got.Compaction.Status != "completed" || !got.Compaction.Continuation || f.input.submits != 2 {
		t.Fatalf("confirmed compaction: %+v submits=%d", got.Compaction, f.input.submits)
	}
}

func TestCompactionRechecksBusyBeforeSubmit(t *testing.T) {
	f, _, p := compactionFixture(t)
	f.cmd.settleInput = func(context.Context, harnessInput, substrate.PaneID, harness.Collar, time.Duration) error {
		f.input.screen = screenWithText("Working (esc to interrupt)", "› /compact")
		return nil
	}
	var ce commandError
	err := f.cmd.compact([]string{"worker"})
	if !errors.As(err, &ce) || ce.status != exitNative || !strings.Contains(err.Error(), "became active") {
		t.Fatalf("race not surfaced: %v", err)
	}
	a, err := p.Read()
	if err != nil {
		t.Fatal(err)
	}
	if f.input.submits != 0 || a.Compaction.Status != "unverified" || a.Compaction.Continuation || a.Input != nil {
		t.Fatalf("unsafe submission: %+v submits=%d", a.Compaction, f.input.submits)
	}
}

func TestCompactionHoldsLegacyResumeUntilConfirmed(t *testing.T) {
	f, a, p := compactionFixture(t)
	start := f.cmd.now()
	a.Compaction = &core.Compaction{ID: "legacy", Status: "submitted", StartedAt: start, Deadline: start.Add(operationTimeout), Continuation: true, Resume: core.Message{Text: "resume"}}
	l, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Save(a); err != nil {
		t.Fatal(err)
	}
	e := core.Envelope{ID: "resume-legacy", Recipient: a.ID, To: a.Name, From: core.Sender{Kind: core.SenderSelfDeclared, Name: "compact"}, Message: a.Compaction.Resume, CreatedAt: start}
	if err := f.run.publishOnce(l, &a, e); err != nil {
		t.Fatal(err)
	}
	l.Close()
	// Expiry releases the normal input gate, but must not release this resume.
	f.run.cmd.clock = func() time.Time { return start.Add(operationTimeout) }
	if err := f.run.tickAgent(a.ID, hookNotice{}, false); err != nil {
		t.Fatal(err)
	}
	if f.input.submits != 0 {
		t.Fatal("legacy continuation bypassed native confirmation")
	}
	if err := f.run.tickAgent(a.ID, hookNotice{Kind: "compaction-finished", SessionID: "s", At: start.Add(time.Second)}, false); err != nil {
		t.Fatal(err)
	}
	if f.input.submits != 1 {
		t.Fatalf("late confirmation did not release resume: %d", f.input.submits)
	}
}

func TestCompactionSurfacesNativeRefusalWithoutResume(t *testing.T) {
	for _, delayed := range []bool{false, true} {
		t.Run(map[bool]string{false: "immediate", true: "later tick"}[delayed], func(t *testing.T) {
			f, a, p := compactionFixture(t)
			refusal := "'/compact' is disabled while a task is in progress."
			submit := f.input.submit
			f.input.submit = func(prompt string) error {
				if strings.HasPrefix(prompt, "/compact") && !delayed {
					f.input.screen = screenWithText(refusal, "› ")
				}
				return submit(prompt)
			}
			err := f.cmd.compact([]string{"worker"})
			if delayed {
				f.input.screen = screenWithText(refusal, "› ")
				err = f.run.tickAgent(a.ID, hookNotice{}, false)
			}
			var ce commandError
			if !errors.As(err, &ce) || ce.status != exitNative || !strings.Contains(err.Error(), refusal) {
				t.Fatalf("refusal not surfaced: %v", err)
			}
			got, err := p.Read()
			if err != nil {
				t.Fatal(err)
			}
			pending, err := p.ListNew()
			if err != nil {
				t.Fatal(err)
			}
			if got.Compaction.Status != "failed" || got.Compaction.Continuation || len(pending) != 0 || f.input.submits != 1 {
				t.Fatalf("refusal resumed: %+v pending=%d submits=%d", got.Compaction, len(pending), f.input.submits)
			}
		})
	}
}

func TestCompactionConfirmationDeadlineIsExplicit(t *testing.T) {
	for _, budget := range []time.Duration{time.Millisecond, time.Second, time.Hour} {
		start := time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC)
		a := core.Agent{ID: "a", Status: core.Active, Activity: core.Compacting, Compaction: &core.Compaction{ID: "c", Status: "submitted", StartedAt: start, Deadline: start.Add(budget)}}
		before, _ := core.Step(a, core.Event{Type: "deadline_checked", HitchID: a.ID, At: start.Add(budget - time.Nanosecond)})
		after, _ := core.Step(a, core.Event{Type: "deadline_checked", HitchID: a.ID, At: start.Add(budget)})
		if before.Compaction.Status != "submitted" || after.Compaction.Status != "unverified" || !strings.Contains(after.Evidence, "unconfirmed") {
			t.Fatalf("budget=%s before=%+v after=%+v", budget, before.Compaction, after.Compaction)
		}
	}
}
