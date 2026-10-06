package main

import (
	"errors"
	"testing"
	"time"

	"github.com/adambiggs/gangline/store"
)

func TestRenameWaitsForAgentLock(t *testing.T) {
	for _, exhausted := range []bool{false, true} {
		t.Run(map[bool]string{false: "released", true: "exhausted"}[exhausted], func(t *testing.T) {
			f := newStateFixture(t)
			a := f.add(t, "a", "worker", "codex")
			p, _ := f.run.team.Agent(a.ID)
			l, err := p.TryLock()
			if err != nil {
				t.Fatal(err)
			}
			defer l.Close()
			elapsed := lockClock(t, f, func(d time.Duration) {
				got, err := p.Read()
				if err != nil || got.Name != a.Name || got.RenameTo != "" {
					t.Fatalf("rename changed state before acquiring lock: %+v %v", got, err)
				}
				if !exhausted && d >= 70*time.Millisecond {
					if err := l.Close(); err != nil {
						t.Fatal(err)
					}
				}
			})
			err = f.cmd.rename([]string{"worker", "renamed"})
			wantName, wantElapsed := a.Name, agentLockBudget
			if exhausted {
				if !errors.Is(err, store.ErrLocked) {
					t.Fatalf("held lock: %v", err)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				wantName, wantElapsed = "renamed", 70*time.Millisecond
			}
			// Release succeeds with 430ms of budget left; exhaustion has no overshoot.
			if *elapsed != wantElapsed {
				t.Fatalf("elapsed = %s, want %s", *elapsed, wantElapsed)
			}
			got, err := p.Read()
			if err != nil || got.Name != wantName || got.RenameFrom != "" || got.RenameTo != "" {
				t.Fatalf("rename state: %+v %v", got, err)
			}
			id, err := f.run.team.ResolveName(string(wantName))
			if err != nil || id != a.ID {
				t.Fatalf("name claim = %s %v", id, err)
			}
		})
	}
}
