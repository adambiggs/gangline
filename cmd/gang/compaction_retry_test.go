package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/adambiggs/gangline/store"
)

func TestCompactionRetryWaitsForNativeTurnCompletion(t *testing.T) {
	f, a, p := compactionFixture(t)
	start := f.cmd.now()
	if err := p.WriteWitness(store.Witness{ID: "w", At: start, SessionID: "s", TurnID: "open"}); err != nil {
		t.Fatal(err)
	}
	f.tickAt(t, a, start, hookNotice{Kind: "turn-started", TurnID: "open"})
	if err := f.cmd.compact([]string{"worker", "--resume", "saved retry note"}); err != nil {
		t.Fatal(err)
	}
	queued := readAgent(t, f, a.ID)
	// An idle-looking screen previously triggered compaction before the
	// native boundary, which could reject it while the turn still ran.
	for _, at := range []time.Time{start.Add(time.Second), start.Add(time.Minute)} {
		got := f.tickAt(t, a, at, hookNotice{})
		if got.Compaction.Status != "queued" || got.Compaction.ID != queued.Compaction.ID || got.Compaction.Resume.Text != "saved retry note" || f.input.submits != 0 {
			t.Fatalf("open turn retry: compaction=%+v submits=%d", got.Compaction, f.input.submits)
		}
	}
	got := f.tickAt(t, a, start.Add(2*time.Minute), hookNotice{Kind: "turn-finished", TurnID: "open"})
	if got.Compaction.Status != "submitted" || got.Compaction.ID != queued.Compaction.ID || got.Compaction.Resume.Text != "saved retry note" || f.input.submits != 2 {
		t.Fatalf("finished turn retry: compaction=%+v submits=%d", got.Compaction, f.input.submits)
	}
}

func TestCompactionDoesNotQueueResumeBehindOrdinaryBusyTask(t *testing.T) {
	f, _, p := compactionFixture(t)
	f.cmd.newTimeout = func(ctx context.Context, d time.Duration) (context.Context, context.CancelFunc) {
		if d == compactStartWindow {
			spent, cancel := context.WithCancel(ctx)
			cancel()
			return spent, cancel
		}
		return context.WithCancel(ctx)
	}
	f.input.submit = func(prompt string) error {
		if prompt == "/compact" {
			f.input.screen = screenWithText("Working (esc to interrupt)", "› ")
		}
		return nil
	}
	_ = f.cmd.compact([]string{"worker", "--resume", "saved retry note"})
	a, err := p.Read()
	if err != nil || a.Compaction.Status != "failed" || !strings.Contains(a.Compaction.Reason, "no compaction showed") || f.input.submits != 1 || a.Compaction.Resume.Text != "saved retry note" {
		t.Fatalf("ordinary task: compaction=%+v submits=%d err=%v", a.Compaction, f.input.submits, err)
	}
}
