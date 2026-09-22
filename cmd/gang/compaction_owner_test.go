package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/store"
)

func compactionOwnerFixture(t *testing.T) (*runtime, core.CompactionRequested) {
	t.Helper()
	root := t.TempDir()
	run := &runtime{cmd: command{getenv: func(string) string { return "" }}, settings: settings{Session: "compact-owner-test", StateRoot: root, Socket: filepath.Join(root, "absent.sock")}}
	now := time.Now()
	_, _, _, err := run.appendEvent(core.AdoptRequested{At: now, Pane: "%1", Hitch: core.Hitch{ID: "worker", Name: "worker", Collar: "claude-code", Directory: root}}, map[core.HitchID]*os.File{})
	if err != nil {
		t.Fatal(err)
	}
	return run, core.CompactionRequested{At: now, Compaction: core.Compaction{ID: "c-1", HitchID: "worker", Resume: core.Message{Text: "continue"}, Deadline: now.Add(time.Minute)}}
}

func TestCompactionIntentClaimsInputBeforePublication(t *testing.T) {
	run, request := compactionOwnerFixture(t)
	held := make(map[core.HitchID]*os.File)
	defer func() {
		for _, owner := range held {
			owner.Close()
		}
	}()
	_, _, effects, err := run.appendEvent(request, held)
	if err != nil {
		t.Fatal(err)
	}
	if len(effects) != 1 || held["worker"] == nil {
		t.Fatalf("compaction intent published without ownership: effects=%v owners=%v", effects, held)
	}
	other, err := run.paths().LockInput(run.settings.Session, "worker")
	if !errors.Is(err, store.ErrLocked) {
		if other != nil {
			other.Close()
		}
		t.Fatalf("compaction native input was not exclusive: %v", err)
	}
}

func TestCompactionRecoveryDoesNotReplayLiveSubmittedOrAbandonedInput(t *testing.T) {
	for _, submitted := range []bool{false, true} {
		run, request := compactionOwnerFixture(t)
		held := make(map[core.HitchID]*os.File)
		_, _, effects, err := run.appendEvent(request, held)
		if err != nil {
			t.Fatal(err)
		}
		owner := held["worker"]
		if owner == nil {
			t.Fatal("compaction has no input owner")
		}
		t.Cleanup(func() { owner.Close() })
		effect := effects[0].(core.CompactHitch)
		state, err := run.recoverCompaction(effect)
		if err != nil || state.Compactions["c-1"].Status != core.CompactionRunning {
			t.Fatalf("recovery disturbed live owner: %v %v", state.Compactions, err)
		}
		if submitted {
			if _, _, _, err := run.appendEvent(core.CompactionSubmitted{At: time.Now(), CompactionID: "c-1"}, held); err != nil {
				t.Fatal(err)
			}
		}
		if err := owner.Close(); err != nil {
			t.Fatal(err)
		}
		want := core.CompactionUnverified
		if submitted {
			want = core.CompactionRunning
		}
		for i := 0; i < 2; i++ {
			state, err = run.recoverCompaction(effect)
			if err != nil || state.Compactions["c-1"].Status != want {
				t.Fatalf("submitted=%t recovery %d replayed or lost input: %v %v", submitted, i, state.Compactions, err)
			}
		}
		if !submitted && len(core.PendingEffects(state)) != 0 {
			t.Fatal("abandoned uncertain input remains executable")
		}
	}
}

func TestContendedCompactionDoesNotPublishExecutableInput(t *testing.T) {
	run, request := compactionOwnerFixture(t)
	owner, err := run.paths().LockInput(run.settings.Session, "worker")
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	_, state, effects, err := run.appendEvent(request, map[core.HitchID]*os.File{})
	if err != nil {
		t.Fatal(err)
	}
	if len(effects) != 0 || state.Compactions["c-1"].Status != core.CompactionFailed {
		t.Fatalf("contended compaction remained executable: %v %v", state.Compactions, effects)
	}
	replayed, err := run.load()
	if err != nil || replayed.Compactions["c-1"].Status != core.CompactionFailed {
		t.Fatalf("compaction refusal was not durable: %v %v", replayed.Compactions, err)
	}
}

func TestConcurrentCompactionCompletionsPersistOneContinuation(t *testing.T) {
	run, request := compactionOwnerFixture(t)
	held := make(map[core.HitchID]*os.File)
	if _, _, _, err := run.appendEvent(request, held); err != nil {
		t.Fatal(err)
	}
	defer held["worker"].Close()
	now := time.Now()
	continuation := core.Envelope{ID: "resume-c-1", From: core.Sender{Kind: core.SenderSelfDeclared, Name: "compact"}, To: "worker", Message: request.Compaction.Resume, CreatedAt: now}
	completion := core.CompactionCompleted{At: now, CompactionID: "c-1", Continuation: &continuation}
	start := make(chan struct{})
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() {
			<-start
			_, _, _, err := run.appendEvent(completion, make(map[core.HitchID]*os.File))
			results <- err
		}()
	}
	close(start)
	for i := 0; i < 2; i++ {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
	state, err := run.load()
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Deliveries) != 1 || len(state.DeliveryOrder) != 1 || state.Deliveries["resume-c-1"].Status != core.DeliveryQueued || state.Compactions["c-1"].Status != core.CompactionSucceeded {
		t.Fatalf("concurrent completion lost or duplicated durable continuation: compact=%v deliveries=%v", state.Compactions, state.Deliveries)
	}
}
