package main

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/store"
)

// A compaction that outlives its deadline is logged as unverified, and the
// roster keeps reading it as compacting while the pane shows the compaction.
func TestCompactionPastDeadlineIsLoggedUnverified(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "claude")
	a.Activity = core.Compacting
	a.Compaction = &core.Compaction{ID: "c", Status: "submitted", Continuation: true, StartedAt: f.cmd.now().Add(-time.Minute), Deadline: f.cmd.now().Add(-time.Second)}
	p, _ := f.run.team.Agent(a.ID)
	l, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Save(a); err != nil {
		t.Fatal(err)
	}
	l.Close()
	f.input.screen = screenWithText("✻ Compacting conversation… (41s · ↓ 3.0k tokens)", "────────", "❯ ", "────────")
	if err := f.cmd.tick([]string{"--agent", "worker"}); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(f.run.team.Log)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	var unverified []core.Event
	if err := store.ReadLog(file, func(e core.Event) error {
		if e.Type == "compaction_unverified" {
			unverified = append(unverified, e)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(unverified) != 1 || unverified[0].ID != "c" || unverified[0].Reason != core.CompactionUnconfirmed {
		t.Fatalf("logged moves to unverified: %+v", unverified)
	}
	got, err := p.Read()
	if err != nil || got.Compaction.Status != "unverified" {
		t.Fatalf("compaction after tick: %+v, %v", got.Compaction, err)
	}
	f.out.Reset()
	if err := f.cmd.status([]string{"worker"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(f.out.String(), "\tcompacting") {
		t.Fatalf("roster during an unverified compaction: %q", f.out)
	}
}
