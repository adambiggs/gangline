//go:build acceptance

package store

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
)

func TestClearAdmissionIsOneTimeWhileDeliveryLockHeld(t *testing.T) {
	team := testTeam(t)
	a := testAgent("a", "worker")
	a.Registration.Generation = "launch"
	now := time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC)
	a.Native.SessionID = "old"
	a.Clear = &core.ClearRequest{Envelope: core.Envelope{ID: "clear-one"}, PreviousSession: "old", Status: "continuing", StartedAt: now}
	l, err := team.CreateAgent(a)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	if err := l.Paths.WriteSessionStart(SessionStart{At: now, SessionID: "new", Transcript: "new.jsonl", Source: "clear", Registration: "launch"}); err != nil {
		t.Fatal(err)
	}
	var admitted atomic.Int32
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			if l.Paths.AdmitClear("clear-one", "new", "launch") == nil {
				admitted.Add(1)
			}
		})
	}
	wg.Wait()
	if admitted.Load() != 1 {
		t.Fatalf("admitted %d copies", admitted.Load())
	}
	proof, err := l.Paths.ReadClearAdmission()
	if err != nil || proof.ID != "clear-one" || proof.SessionID != "new" {
		t.Fatalf("proof=%+v err=%v", proof, err)
	}
	if err := l.Paths.WriteSessionStart(SessionStart{At: now, SessionID: "foreign", Transcript: "foreign.jsonl", Source: "clear", Registration: "old-launch"}); err == nil {
		t.Fatal("old registration authorized clear")
	}
}

func TestClearProofAndDisplayRejectLateOldHooks(t *testing.T) {
	team := testTeam(t)
	a := testAgent("a", "worker")
	a.Registration.Generation = "launch"
	now := time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC)
	a.Native.SessionID = "old"
	a.Clear = &core.ClearRequest{Envelope: core.Envelope{ID: "clear-one"}, PreviousSession: "old", Status: "continuing", StartedAt: now}
	l, err := team.CreateAgent(a)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	p := l.Paths
	if err := p.WriteSessionStart(SessionStart{At: now, SessionID: "new", Transcript: "new.jsonl", Source: "clear", Registration: "launch"}); err != nil {
		t.Fatal(err)
	}
	if err := p.WriteSessionStart(SessionStart{At: now.Add(time.Hour), SessionID: "old", Transcript: "old.jsonl", Source: "startup", Registration: "launch"}); err == nil {
		t.Fatal("old start replaced clear proof")
	}
	if err := p.WriteDisplayHook(StatusHook{Kind: "permission-requested", SessionID: "new", At: now}, nil, "old", nil); err != nil {
		t.Fatal(err)
	}
	count := 3
	if err := p.WriteDisplayHook(StatusHook{Kind: "turn-finished", SessionID: "old", At: now.Add(time.Hour)}, &count, "old", nil); err != nil {
		t.Fatal(err)
	}
	if w, ok, err := p.ReadPermissionWitness(); err != nil || !ok || w.SessionID != "new" {
		t.Fatalf("late old hook removed permission: %+v %v %v", w, ok, err)
	}
	if _, ok, err := p.ReadBackground(); err != nil || ok {
		t.Fatalf("old background retained: %v %v", ok, err)
	}
	if err := p.WriteDisplayHook(StatusHook{Kind: "turn-started", SessionID: "new", At: now}, nil, "old", nil); err != nil {
		t.Fatal(err)
	}
	if err := p.WriteDisplayHook(StatusHook{Kind: "permission-requested", SessionID: "old", At: now}, nil, "old", nil); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := p.ReadPermissionWitness(); err != nil || ok {
		t.Fatalf("old permission restored: %v %v", ok, err)
	}
}
