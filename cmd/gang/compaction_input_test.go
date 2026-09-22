package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/substrate"
)

type compactInputBackend struct {
	keys            []substrate.Keys
	captures        int
	afterInputError error
}

func (backend *compactInputBackend) Capture(context.Context, substrate.PaneID) (substrate.Screen, error) {
	backend.captures++
	if backend.captures > 2 {
		return substrate.Screen{}, errors.New("fixture exhausted: native Enter never arrived")
	}
	if len(backend.keys) == 0 {
		return screenWithText("────────────────────", "❯ ", "────────────────────"), nil
	}
	if backend.afterInputError != nil {
		return substrate.Screen{}, backend.afterInputError
	}
	// Claude collapses bracketed multiline paste; the native command bytes
	// remain in its composer even though exact readback cannot see them.
	return screenWithText("────────────────────", "❯ [Pasted text #1 +2 lines]", "────────────────────"), nil
}

func (backend *compactInputBackend) ForegroundProcesses(context.Context, substrate.PaneID) ([]substrate.Process, error) {
	return []substrate.Process{{PID: 42, Command: "claude"}}, nil
}

func (backend *compactInputBackend) SendKeys(_ context.Context, _ substrate.PaneID, keys substrate.Keys) error {
	backend.keys = append(backend.keys, keys)
	return nil
}

func compactInputFixture(t *testing.T) (*runtime, core.State, core.CompactHitch) {
	t.Helper()
	root := t.TempDir()
	collar, err := os.ReadFile("../../harness/collars/claude-code.cue")
	if err != nil {
		t.Fatal(err)
	}
	collar = []byte(strings.Replace(string(collar), `settle: "400ms"`, `settle: "0s"`, 1))
	if err := os.WriteFile(filepath.Join(root, "claude-code.cue"), collar, 0600); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	hitch := core.Hitch{ID: "worker", Name: "worker", Collar: "claude-code", Directory: root, Status: core.HitchActive, Activity: core.ActivityCompacting, Pane: "%1", PendingCompactID: "c-1"}
	compact := core.Compaction{ID: "c-1", HitchID: hitch.ID, Resume: core.Message{Text: "first line\nsecond line\nthird line"}, Deadline: now.Add(time.Minute), Status: core.CompactionRunning}
	state := core.NewState(core.Team{ID: "team", Name: "team"})
	state.Hitches[hitch.ID] = hitch
	state.Compactions[compact.ID] = compact
	return &runtime{settings: settings{CollarDir: root}}, state, core.CompactHitch{Compaction: compact, Pane: hitch.Pane}
}

func TestCompactionSubmitsCollapsedClaudePaste(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		run, state, effect := compactInputFixture(t)
		backend := &compactInputBackend{}
		outcome, err := run.compactHitch(state, backend, effect)
		if err != nil {
			t.Fatal(err)
		}
		if len(backend.keys) != 2 || !backend.keys[1].Submit {
			t.Fatalf("collapsed paste lost Enter: keys=%v outcome=%#v", backend.keys, outcome)
		}
		if want := "\x1b[200~/compact " + effect.Compaction.Resume.Text + "\x1b[201~"; backend.keys[0].Text != want {
			t.Fatalf("compaction ignored collar paste semantics: %q", backend.keys[0].Text)
		}
		if _, ok := outcome.(core.CompactionSubmitted); !ok {
			t.Fatalf("native Enter was not recorded separately from completion: %#v", outcome)
		}
	})
}

func TestCompactionCaptureFailureAfterInputRemainsUnverified(t *testing.T) {
	run, state, effect := compactInputFixture(t)
	backend := &compactInputBackend{afterInputError: errors.New("capture disconnected after paste")}
	outcome, err := run.compactHitch(state, backend, effect)
	unknown, ok := outcome.(core.CompactionUnverifiedEvent)
	if err != nil || !ok || !strings.Contains(unknown.Evidence, "capture disconnected") || len(backend.keys) != 1 {
		t.Fatalf("partial compaction was treated as safe to retry: keys=%v outcome=%#v err=%v", backend.keys, outcome, err)
	}
	state, _ = core.Step(state, outcome)
	if state.Compactions["c-1"].Status != core.CompactionUnverified || len(core.PendingEffects(state)) != 0 {
		t.Fatalf("uncertain compact input remained executable: %v", state.Compactions)
	}
}
