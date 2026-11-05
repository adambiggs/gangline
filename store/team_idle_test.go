//go:build acceptance

package store

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
)

func idleStoreFixture(t *testing.T) (TeamPaths, *LockedAgent, core.Agent) {
	t.Helper()
	p := testTeam(t)
	a := core.Agent{ID: "worker", Name: "worker"}
	l, err := p.CreateAgent(a)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.Close() })
	return p, l, a
}
func readIdleFixture(t *testing.T, p TeamPaths) IdleState {
	t.Helper()
	l, err := p.LockIdle()
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	s, err := l.Read()
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func TestIdlePublicationRecoveryAndPruning(t *testing.T) {
	p, l, a := idleStoreFixture(t)
	e := core.Envelope{ID: "one", Recipient: a.ID}
	// Crash before inbox publication: intent does not establish work.
	lock, err := l.Paths.prepareIdlePublication(e)
	if err != nil {
		t.Fatal(err)
	}
	_ = lock.Close()
	if got := readIdleFixture(t, p).Work[a.ID]; got != 0 {
		t.Fatalf("pre-link crash armed %d", got)
	}
	// A successful link has no post-link metadata write to fail.
	if err := l.Paths.PublishNew(e); err != nil {
		t.Fatal(err)
	}
	if err := l.Settle(&a, e, "delivered", ""); err != nil {
		t.Fatal(err)
	}
	if got := readIdleFixture(t, p).Work[a.ID]; got != 1 {
		t.Fatalf("published work=%d", got)
	}
	// A later ordinary receipt prunes the first, without forgetting its work.
	e2 := core.Envelope{ID: "two", Recipient: a.ID}
	if err := l.Paths.PublishNew(e2); err != nil {
		t.Fatal(err)
	}
	if err := l.Settle(&a, e2, "delivered", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := l.Paths.ReadEnvelope("cur", e.ID); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("old receipt retained: %v", err)
	}
	if got := readIdleFixture(t, p).Work[a.ID]; got != 2 {
		t.Fatalf("pruning lost work: %d", got)
	}
}
func TestIdlePublicationDuplicateAndConcurrentPublishers(t *testing.T) {
	p, l, a := idleStoreFixture(t)
	e := core.Envelope{ID: "same", Recipient: a.ID}
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() { defer wg.Done(); results <- l.Paths.PublishNew(e) }()
	}
	wg.Wait()
	close(results)
	ok, duplicate := 0, 0
	for err := range results {
		if err == nil {
			ok++
		} else if errors.Is(err, ErrEnvelopeQueued) {
			duplicate++
		} else {
			t.Fatal(err)
		}
	}
	if ok != 1 || duplicate != 1 {
		t.Fatalf("success=%d duplicate=%d", ok, duplicate)
	}
	if got := readIdleFixture(t, p).Work[a.ID]; got != 1 {
		t.Fatalf("duplicate rearmed: %d", got)
	}
	// A retry after resolution must also leave the count alone.
	if err := l.Paths.PublishNew(e); !errors.Is(err, ErrEnvelopeQueued) {
		t.Fatal(err)
	}
	if got := readIdleFixture(t, p).Work[a.ID]; got != 1 {
		t.Fatal(got)
	}
}
func TestIdlePublicationMetadataFailureDoesNotQueue(t *testing.T) {
	p, l, a := idleStoreFixture(t)
	if err := os.Mkdir(filepath.Join(p.Directory, "idle.json"), 0700); err != nil {
		t.Fatal(err)
	}
	e := core.Envelope{ID: "failed", Recipient: a.ID}
	if err := l.Paths.PublishNew(e); err == nil {
		t.Fatal("corrupt metadata accepted")
	}
	if _, err := l.Paths.ReadEnvelope("new", e.ID); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("envelope published: %v", err)
	}
}

func TestIdlePublicationWithdrawalAndDroppedCounterCleanup(t *testing.T) {
	p, l, a := idleStoreFixture(t)
	e := core.Envelope{ID: "withdrawn", Recipient: a.ID}
	if err := l.Paths.PublishNew(e); err != nil {
		t.Fatal(err)
	}
	if err := l.Withdraw(e.ID); err != nil {
		t.Fatal(err)
	}
	if got := readIdleFixture(t, p).Work[a.ID]; got != 1 {
		t.Fatalf("withdrawal lost accepted publication: %d", got)
	}
	if err := os.Remove(l.Paths.State); err != nil {
		t.Fatal(err)
	}
	if s := readIdleFixture(t, p); len(s.Work) != 0 {
		t.Fatalf("dropped counters retained: %+v", s.Work)
	}
}

func TestIdleWindowDoesNotChangePublicationRecords(t *testing.T) {
	p, l, a := idleStoreFixture(t)
	lock, err := p.LockIdle()
	if err != nil {
		t.Fatal(err)
	}
	w := &IdleWindow{Since: time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC), Last: time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC), Minimum: time.Minute, Members: []IdleWindowMember{{Member: IdleMember{ID: a.ID}}}}
	if err := lock.SaveWindow(w); err != nil {
		t.Fatal(err)
	}
	if got := lock.ReadWindow(); got == nil || got.Minimum != time.Minute {
		t.Fatalf("window=%+v", got)
	}
	_ = lock.Close()
	// Ordinary publication reads only the unchanged strict ledger, even when
	// the optional timer cache is corrupt or unavailable.
	for i, data := range []string{"{", `{"future_field":true}`} {
		if err := os.WriteFile(filepath.Join(p.Directory, "idle-window.json"), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
		e := core.Envelope{ID: core.EnvelopeID(fmt.Sprintf("work-%d", i)), Recipient: a.ID}
		if err := l.Paths.PublishNew(e); err != nil {
			t.Fatal(err)
		}
		if s := readIdleFixture(t, p); s.Work[a.ID] != uint64(i+1) {
			t.Fatalf("work=%v", s.Work)
		}
	}
	lock, err = p.LockIdle()
	if err != nil {
		t.Fatal(err)
	}
	if lock.ReadWindow() != nil {
		t.Fatal("invalid cache trusted")
	}
	if err := lock.SaveWindow(nil); err != nil {
		t.Fatal(err)
	}
	_ = lock.Close()
	if _, err := os.Stat(filepath.Join(p.Directory, "idle-window.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("cache retained")
	}
}
