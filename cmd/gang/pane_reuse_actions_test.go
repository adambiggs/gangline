package main

import (
	"os"
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
