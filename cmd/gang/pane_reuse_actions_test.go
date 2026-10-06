package main

import (
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/store"
	"github.com/adambiggs/gangline/substrate/tmux"
)

// staleOn points the record at the reused pane id under the registration of
// an earlier server, as a record left by an ended server reads.
func staleOn(reg tmux.PaneIdentity, earlier string) func(*core.Agent) {
	return func(a *core.Agent) {
		a.Pane = reg.Pane
		a.Registration.Generation, a.Registration.Session = earlier, reg.Session
	}
}

// requireReplacedPaneRefusal requires a refusal that names the replaced pane
// and the step that frees the agent's name.
func requireReplacedPaneRefusal(t *testing.T, err error) {
	t.Helper()
	var ce commandError
	if !errors.As(err, &ce) || ce.status != exitRefused || !strings.Contains(ce.text, tmux.ErrPaneReplaced.Error()) || !strings.Contains(ce.text, "drop worker and hitch it again") {
		t.Fatalf("err = %v, want a refusal naming the replaced pane and the next step", err)
	}
}

func loggedEvents(t *testing.T, f *stateFixture, kind string) int {
	t.Helper()
	file, err := os.Open(f.run.team.Log)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	n := 0
	if err := store.ReadLog(file, func(e core.Event) error {
		if e.Type == kind {
			n++
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return n
}

// Startup recovery reopens a failed boot only from the record's own pane: a
// ready composer in a pane whose id a stale record names belongs to another
// agent and says nothing of this startup.
func TestRecoverDoesNotReopenBootFromAPaneWhoseIdAStaleRecordReuses(t *testing.T) {
	f := newStateFixture(t)
	a, p, _ := failAtBootDeadline(t, f, "")
	reg, earlier := reusedPane(t, f)
	f.setAgent(t, a, staleOn(reg, earlier))
	f.input.screen = screenWithText("READY", "› ")
	f.input.captures = 0
	err := f.cmd.hitch([]string{"worker", "--recover"})
	if err == nil || !strings.Contains(err.Error(), tmux.ErrPaneReplaced.Error()) {
		t.Fatalf("recover err = %v, want the replaced pane", err)
	}
	got, err := p.Read()
	if err != nil || got.Status != core.Failed || got.Evidence != core.BootDeadlineElapsed {
		t.Fatalf("stale record after recover: %+v err=%v", got, err)
	}
	if n := loggedEvents(t, f, "boot_reopened"); n != 0 || f.input.captures != 0 || f.input.submits != 0 {
		t.Fatalf("boot_reopened=%d captures=%d submits=%d", n, f.input.captures, f.input.submits)
	}
}

// Compaction recovery reads only the record's own pane: another agent's busy
// screen under a reused pane id is not the compaction, and no recovery is
// recorded against it.
func TestCompactRecoverDoesNotReadAPaneWhoseIdAStaleRecordReuses(t *testing.T) {
	f := newStateFixture(t)
	reg, earlier := reusedPane(t, f)
	a := f.add(t, "a", "worker", "codex")
	f.input.screen = compactInterruptScreens(t, f, "codex").busy
	before := saveRecoverCompaction(t, f, a, "submitted", staleOn(reg, earlier))
	requireReplacedPaneRefusal(t, f.cmd.compact([]string{"worker", "--interrupt"}))
	p, _ := f.run.team.Agent(a.ID)
	after, err := p.Read()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(after.Compaction, before.Compaction) || after.Input != nil || f.input.captures != 0 || len(f.input.keys) != 0 {
		t.Fatalf("compaction=%+v input=%+v captures=%d keys=%v", after.Compaction, after.Input, f.input.captures, f.input.keys)
	}
}

// A compaction starts only from the record's own pane: another agent's idle
// screen under a reused pane id is not this agent's activity.
func TestCompactDoesNotReadAPaneWhoseIdAStaleRecordReuses(t *testing.T) {
	f := newStateFixture(t)
	reg, earlier := reusedPane(t, f)
	a := f.setAgent(t, f.add(t, "a", "worker", "codex"), func(a *core.Agent) {
		staleOn(reg, earlier)(a)
		a.Activity = core.Busy
	})
	f.input.screen = screenWithText("READY", "› ")
	err := f.cmd.compact([]string{"worker"})
	if !errors.Is(err, tmux.ErrPaneReplaced) {
		t.Fatalf("compact err = %v, want the replaced pane", err)
	}
	p, _ := f.run.team.Agent(a.ID)
	got, err := p.Read()
	if err != nil {
		t.Fatal(err)
	}
	if got.Activity == core.Idle || got.Compaction == nil || got.Compaction.Status != "queued" || f.input.captures != 0 || len(f.input.keys) != 0 {
		t.Fatalf("activity=%s compaction=%+v captures=%d keys=%v", got.Activity, got.Compaction, f.input.captures, f.input.keys)
	}
}

// Capture shows only the record's own pane: a stale record whose pane id
// another agent's pane reuses has no screen to show.
func TestCaptureDoesNotReadAPaneWhoseIdAStaleRecordReuses(t *testing.T) {
	for _, c := range []struct {
		name string
		run  func(command) error
	}{
		{"capture", func(cmd command) error { return cmd.capture([]string{"worker"}) }},
		{"composer", func(cmd command) error { return cmd.capture([]string{"worker", "--composer"}) }},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := newStateFixture(t)
			reg, earlier := reusedPane(t, f)
			f.setAgent(t, f.add(t, "a", "worker", "claude"), staleOn(reg, earlier))
			f.input.command = "claude"
			f.input.screen = screenWithText("────────", "❯ ", "────────")
			requireReplacedPaneRefusal(t, c.run(f.cmd))
			if f.input.captures != 0 {
				t.Fatalf("captured another record's pane %d times", f.input.captures)
			}
		})
	}
}

// A record with a pane but an incomplete registration is refused as such
// before any read, rather than as an invalid pane identity.
func TestOperatorReadsRefuseAnIncompleteRegistration(t *testing.T) {
	for _, c := range []struct {
		name string
		run  func(command) error
	}{
		{"compact interrupt", func(cmd command) error { return cmd.compact([]string{"worker", "--interrupt"}) }},
		{"capture", func(cmd command) error { return cmd.capture([]string{"worker"}) }},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := newStateFixture(t)
			a := f.add(t, "a", "worker", "codex")
			f.input.screen = compactInterruptScreens(t, f, "codex").busy
			saveRecoverCompaction(t, f, a, "submitted", func(a *core.Agent) { a.Registration.Generation = "" })
			err := c.run(f.cmd)
			var ce commandError
			if !errors.As(err, &ce) || ce.status != exitRefused || !strings.Contains(ce.text, "incomplete pane registration") {
				t.Fatalf("err = %v, want the incomplete registration refusal", err)
			}
			if f.input.captures != 0 || len(f.input.keys) != 0 {
				t.Fatalf("captures=%d keys=%v", f.input.captures, f.input.keys)
			}
		})
	}
}
