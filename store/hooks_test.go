package store

import (
	"github.com/adambiggs/gangline/core"
	"sync"
	"testing"
	"time"
)

func TestStatusHooksBoundedOrderedAndIndependentOfAgentLock(t *testing.T) {
	team, err := (Paths{Root: t.TempDir()}).Team("hooks")
	if err != nil {
		t.Fatal(err)
	}
	if err := team.Create(); err != nil {
		t.Fatal(err)
	}
	l, err := team.CreateAgent(core.Agent{ID: "a", Name: "worker", Collar: "codex", Status: core.Active})
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	now := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	kinds := []string{"turn-started", "turn-finished", "turn-failed", "activity", "permission-requested", "compaction-started", "compaction-finished"}
	var wg sync.WaitGroup
	for _, kind := range kinds {
		wg.Go(func() {
			if err := l.Paths.WriteStatusHook(StatusHook{Kind: kind, At: now}); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if err := l.Paths.WriteStatusHook(StatusHook{Kind: "activity", At: now.Add(time.Second)}); err != nil {
		t.Fatal(err)
	}
	if err := l.Paths.WriteStatusHook(StatusHook{Kind: "activity", At: now.Add(-time.Second)}); err != nil {
		t.Fatal(err)
	}
	records, err := l.Paths.ReadStatusHooks()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != len(kinds) {
		t.Fatalf("retained %d records", len(records))
	}
	for i, n := range records {
		if i > 0 && n.Sequence <= records[i-1].Sequence {
			t.Fatalf("out of order: %+v", records)
		}
		if n.Kind == "activity" && !n.At.Equal(now.Add(time.Second)) {
			t.Fatalf("old writer replaced new: %+v", n)
		}
	}
	if records[len(records)-1].Sequence != 8 {
		t.Fatalf("sequence=%d", records[len(records)-1].Sequence)
	}
}

func TestStatusHooksRetainPendingTerminalBoundaries(t *testing.T) {
	team := testTeam(t)
	a := testAgent("a", "worker")
	l, err := team.CreateAgent(a)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	now := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	for _, id := range []string{"new", "old"} {
		if err := l.Paths.WriteStatusHook(StatusHook{Kind: "turn-failed", At: now, TurnID: id}); err != nil {
			t.Fatal(err)
		}
	}
	records, err := l.Paths.ReadStatusHooks()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 {
		t.Fatalf("pending terminal count=%d", len(records))
	}
	a.Native.HookSequence = records[1].Sequence
	if err := l.Save(a); err != nil {
		t.Fatal(err)
	}
	if err := l.Paths.WriteStatusHook(StatusHook{Kind: "turn-failed", At: now, TurnID: "next"}); err != nil {
		t.Fatal(err)
	}
	records, err = l.Paths.ReadStatusHooks()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || records[0].TurnID != "next" {
		t.Fatalf("settled boundaries retained: %+v", records)
	}
}
