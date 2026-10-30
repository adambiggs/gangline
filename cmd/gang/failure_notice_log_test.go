package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adambiggs/gangline/core"
)

func TestFailureNoticeSurvivesAuditAppendFailure(t *testing.T) {
	f := newStateFixture(t)
	lead := f.addHitched(t, "l", "lead", "lead", "")
	child := f.addHitched(t, "c", "child", "", lead.ID)
	cp, err := f.run.team.Agent(child.ID)
	if err != nil {
		t.Fatal(err)
	}
	l, err := cp.LockAgent()
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	log := filepath.Join(f.run.team.Directory, "log.jsonl")
	if err := os.Mkdir(log, 0700); err != nil {
		t.Fatal(err)
	}
	event := core.Event{Type: "hitch_failed", Reason: "native launch failed"}
	applyErr := f.run.apply(l, &child, event)
	if applyErr == nil {
		t.Fatal("audit append failure was not returned")
	}
	if saved := f.agent(t, child.ID); saved.Status != core.Failed {
		t.Fatalf("saved status = %s", saved.Status)
	}
	lp, err := f.run.team.Agent(lead.ID)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Join(lp.Inbox, "new"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("failure notices = %d, want 1", len(entries))
	}
	id := core.EnvelopeID(strings.TrimSuffix(entries[0].Name(), ".json"))
	notice, err := lp.ReadEnvelope("new", id)
	if err != nil || !strings.Contains(notice.Message.Text, event.Reason) {
		t.Fatalf("failure notice = %+v, error = %v", notice, err)
	}
	// Publishing the notice succeeds, but its audit append also fails.
	joined, ok := applyErr.(interface{ Unwrap() []error })
	if !ok || len(joined.Unwrap()) != 2 {
		t.Fatalf("want both audit errors, got %v", applyErr)
	}
	for _, cause := range joined.Unwrap() {
		var pathErr *os.PathError
		if !errors.As(cause, &pathErr) || pathErr.Path != log {
			t.Fatalf("audit error = %v", cause)
		}
	}
	if err := os.Remove(log); err != nil {
		t.Fatal(err)
	}
	if err := f.run.apply(l, &child, event); err != nil {
		t.Fatal(err)
	}
	entries, err = os.ReadDir(filepath.Join(lp.Inbox, "new"))
	if err != nil || len(entries) != 1 {
		t.Fatalf("reapplied event notices = %d, error = %v", len(entries), err)
	}
}
