package main

import (
	"context"
	"errors"
	"fmt"
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
		return nil
	}
	return f, a, p
}

// witnessFailureNotices lets the fixture's native submit hook witness each
// compaction failure notice, which reaches the agent as ordinary input.
// It returns the notices submitted so far.
func witnessFailureNotices(f *stateFixture, p store.AgentPaths) *[]string {
	submit, notices := f.input.submit, &[]string{}
	f.input.submit = func(prompt string) error {
		if strings.HasPrefix(prompt, "[gang:compact#") && strings.Contains(prompt, " failed: ") {
			*notices = append(*notices, prompt)
			return p.WriteWitness(store.Witness{ID: fmt.Sprintf("failure-notice-%d", len(*notices)), At: f.cmd.now(), Prompt: prompt, SessionID: "s"})
		}
		if submit != nil {
			return submit(prompt)
		}
		return nil
	}
	return notices
}

// failureNotice reads the failure notice for compaction id from dir.
func failureNotice(t *testing.T, p store.AgentPaths, dir, id string) core.Envelope {
	t.Helper()
	e, err := p.ReadEnvelope(dir, core.EnvelopeID("failed-"+id))
	if err != nil {
		t.Fatalf("compaction failure notice in %s: %v", dir, err)
	}
	if e.From != (core.Sender{Kind: core.SenderGangline, Name: "compact"}) || !strings.Contains(e.Message.Text, "Compaction "+id+" failed: ") {
		t.Fatalf("compaction failure notice: %+v", e)
	}
	return e
}

func mustEnvelopeText(t *testing.T, e core.Envelope) string {
	t.Helper()
	wire, err := envelopeText(e)
	if err != nil {
		t.Fatal(err)
	}
	return wire
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
	if got.Compaction.Status != "submitted" || f.input.submits != 2 || !got.Compaction.Continuation {
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
	if f.input.submits != 2 || !got.Compaction.Continuation {
		t.Fatalf("premature continuation: %+v submits=%d", got.Compaction, f.input.submits)
	}
	start := got.Compaction.StartedAt
	checkpoint := start.Add(time.Second)
	if err := f.run.tickAgent(a.ID, hookNotice{SessionID: "s", Readings: []core.Reading{{Kind: "compaction-checkpoint", At: &checkpoint}}}, false); err != nil {
		t.Fatal(err)
	}
	if f.input.submits != 2 {
		t.Fatal("checkpoint resumed without completion")
	}
	if err := f.run.tickAgent(a.ID, hookNotice{Kind: "compaction-finished", SessionID: "s", At: start.Add(-time.Second)}, false); err != nil {
		t.Fatal(err)
	}
	if f.input.submits != 2 {
		t.Fatal("stale completion resumed")
	}
	if err := f.run.tickAgent(a.ID, hookNotice{Kind: "compaction-finished", SessionID: "other", At: start.Add(time.Second)}, false); err == nil {
		t.Fatal("foreign completion accepted")
	}
	if f.input.submits != 2 {
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
	// Codex declares no clear keys, so the draft stays and the reason says so.
	if f.input.submits != 0 || a.Compaction.Status != "failed" || a.Compaction.Continuation || a.Input != nil || !strings.Contains(a.Compaction.Reason, "still holds the unsubmitted input") {
		t.Fatalf("unsafe submission: %+v submits=%d", a.Compaction, f.input.submits)
	}
	if notice := failureNotice(t, p, "new", a.Compaction.ID); !strings.Contains(notice.Message.Text, "Your context was not compacted") {
		t.Fatalf("notice: %q", notice.Message.Text)
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
			witnessFailureNotices(f, p)
			err := f.cmd.compact([]string{"worker"})
			if delayed {
				f.input.screen = screenWithText(refusal, "› ")
				if err := f.run.tickAgent(a.ID, hookNotice{}, false); err != nil {
					t.Fatal(err)
				}
			} else if ce := (commandError{}); !errors.As(err, &ce) || ce.status != exitNative || !strings.Contains(err.Error(), refusal) {
				t.Fatalf("refusal not surfaced: %v", err)
			}
			got, err := p.Read()
			if err != nil {
				t.Fatal(err)
			}
			if got.Compaction.Status != "failed" || !got.Compaction.Continuation {
				t.Fatalf("refusal resumed: %+v", got.Compaction)
			}
			if _, err := p.ReadEnvelope("failed", core.EnvelopeID("resume-"+got.Compaction.ID)); err != nil {
				t.Fatalf("resume not withheld: %v", err)
			}
			// The command reports the refusal itself; a later tick reports it
			// only through the queued notice.
			dir, submits := "new", 2
			if delayed {
				dir, submits = "cur", 3
			}
			if notice := failureNotice(t, p, dir, got.Compaction.ID); !strings.Contains(notice.Message.Text, refusal) || !strings.Contains(notice.Message.Text, "Your context was not compacted") || f.input.submits != submits {
				t.Fatalf("failure notice: %+v; submits=%d", notice, f.input.submits)
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

func TestWithheldResumeKeepsRetainedStartupReceipt(t *testing.T) {
	f, a, p := compactionFixture(t)
	at := f.cmd.now()
	retained := core.Envelope{ID: "startup-1", Token: "00112233445566ff", From: core.Sender{Kind: core.SenderGangline, Name: "startup"}, Recipient: a.ID, To: a.Name, Message: core.Message{Text: "contract"}, Purpose: "startup", CreatedAt: at}
	if err := p.Publish(retained); err != nil {
		t.Fatal(err)
	}
	l, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Settle(&a, retained, "unverified", "native input not yet confirmed"); err != nil {
		t.Fatal(err)
	}
	a.Compaction = &core.Compaction{ID: "compact", Resume: core.Message{Text: "continue"}, ResumeToken: "aaaaaaaaaaaaaaaa", ResumeFrom: core.Sender{Kind: core.SenderSelfDeclared, Name: "compact"}, StartedAt: at.Add(-time.Minute), Status: "submitted"}
	if err := l.Save(a); err != nil {
		t.Fatal(err)
	}
	if _, err := f.run.publishCompactionResume(l, &a); err != nil {
		t.Fatal(err)
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.run.confirmCompactionHook(a.ID, hookNotice{Kind: "compaction-finished", SessionID: "s", At: at.Add(time.Second)}); err != nil {
		t.Fatal(err)
	}
	resume, err := p.ReadEnvelope("failed", "resume-compact")
	if err != nil || resume.Outcome != "cancelled" {
		t.Fatalf("withheld resume: %+v, %v", resume, err)
	}
	got, err := p.Read()
	if err != nil {
		t.Fatal(err)
	}
	if _, held, err := retainedStartup(p, "failed", got.LastFailed); err != nil || !held {
		t.Fatalf("withheld resume released the startup hold: LastFailed=%q, %v", got.LastFailed, err)
	}
}
