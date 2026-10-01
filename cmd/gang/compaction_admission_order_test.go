package main

import (
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/store"
)

func recoveredCompaction(t *testing.T) (*stateFixture, core.Agent, store.AgentPaths, string) {
	t.Helper()
	f, a, p := compactionFixture(t)
	a.Compaction = &core.Compaction{
		ID: "c", Resume: core.Message{Text: "continue"}, ResumeToken: "aaaaaaaaaaaaaaaa",
		ResumeFrom: core.Sender{Kind: core.SenderSelfDeclared, Name: "compact"},
		StartedAt:  f.cmd.now().Add(-time.Minute), Deadline: f.cmd.now().Add(time.Minute),
		Status: "unverified", Continuation: true, Recovered: true,
		Reason: "operator recovery sent Escape; native task still active",
	}
	l, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Save(a); err != nil {
		t.Fatal(err)
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	wire, err := envelopeText(core.Envelope{ID: "resume-c", Token: a.Compaction.ResumeToken, From: a.Compaction.ResumeFrom, Message: a.Compaction.Resume, Purpose: "resume"})
	if err != nil {
		t.Fatal(err)
	}
	return f, a, p, wire
}

// An interrupted compaction fires no completion hook, so a recovered
// compaction's note is admitted once without completion proof and stays
// unverified.
func TestRecoveredCompactionAdmitsNoteWithoutCompletion(t *testing.T) {
	f, a, p, wire := recoveredCompaction(t)
	c, err := loadCollar("codex", f.run.settings)
	if err != nil {
		t.Fatal(err)
	}
	if reason, err := f.run.admitCompactionResume(a.ID, c, wire, "s"); err != nil || reason != "" {
		t.Fatalf("recovered note withheld: %q, %v", reason, err)
	}
	got, err := p.Read()
	if err != nil {
		t.Fatal(err)
	}
	if !got.Compaction.ResumeAdmitted || got.Compaction.Status != "unverified" || !got.Compaction.CompletedAt.IsZero() {
		t.Fatalf("admission claimed completion: %+v", got.Compaction)
	}
	if reason, err := f.run.admitCompactionResume(a.ID, c, wire, "s"); err != nil || reason != "compaction continuation already admitted" {
		t.Fatalf("second admission: %q, %v", reason, err)
	}
}

// A completion hook that finds the agent locked leaves its witness on disk;
// admission reconciles that witness before it decides.
func TestCompletionWitnessUnderLockSettlesBeforeAdmission(t *testing.T) {
	f, a, p, wire := recoveredCompaction(t)
	c, err := loadCollar("codex", f.run.settings)
	if err != nil {
		t.Fatal(err)
	}
	held, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	if err := f.run.confirmCompactionHook(a.ID, hookNotice{Kind: "compaction-finished", SessionID: "s", At: f.cmd.now()}); err != nil {
		t.Fatal(err)
	}
	if err := held.Close(); err != nil {
		t.Fatal(err)
	}
	got, err := p.Read()
	if err != nil {
		t.Fatal(err)
	}
	if got.Compaction.Status != "unverified" || !got.Compaction.CompletedAt.IsZero() {
		t.Fatalf("locked hook changed state: %+v", got.Compaction)
	}
	if reason, err := f.run.admitCompactionResume(a.ID, c, wire, "s"); err != nil || reason != "" {
		t.Fatalf("note withheld: %q, %v", reason, err)
	}
	got, err = p.Read()
	if err != nil {
		t.Fatal(err)
	}
	if got.Compaction.Status != "completed" || !got.Compaction.CompletedAt.Equal(f.cmd.now()) || !got.Compaction.ResumeAdmitted {
		t.Fatalf("admission did not settle the witness first: %+v", got.Compaction)
	}
}
