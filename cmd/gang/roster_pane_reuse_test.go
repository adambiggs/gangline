package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/substrate"
	"github.com/adambiggs/gangline/substrate/tmux"
)

// reusedPane starts a private tmux server with one registered pane and
// returns that pane's registration and a generation of an earlier server. A
// new server numbers its panes from the start again, so a record left by an
// ended server can name the id of another agent's live pane.
func reusedPane(t *testing.T, f *stateFixture) (tmux.PaneIdentity, string) {
	t.Helper()
	t.Setenv("TMUX", "")
	t.Setenv("TMUX_PANE", "")
	binary, err := exec.LookPath("tmux")
	if err != nil {
		t.Skip("tmux is required")
	}
	root, err := os.MkdirTemp("", "gang-panereuse-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	socket := root + "/tmux.sock"
	session := f.env["GANG_SESSION"]
	f.env["GANG_TMUX"], f.env["GANG_TMUX_SOCKET"] = binary, socket
	if out, err := exec.Command(binary, "-S", socket, "new-session", "-d", "-s", session, "cat").CombinedOutput(); err != nil {
		t.Fatalf("new-session: %v\n%s", err, out)
	}
	t.Cleanup(func() { _ = exec.Command(binary, "-S", socket, "kill-session", "-t", "="+session).Run() })
	b, err := tmux.New(tmux.Config{Binary: binary, Socket: socket, Session: session})
	if err != nil {
		t.Fatal(err)
	}
	f.cmd.paneBackend = b
	ctx := context.Background()
	pane, err := b.Spawn(ctx, substrate.SpawnSpec{Name: "live", Directory: root, Command: "cat"})
	if err != nil {
		t.Fatal(err)
	}
	reg, err := b.RegisterPane(ctx, pane.ID)
	if err != nil {
		t.Fatal(err)
	}
	earlier := strings.Repeat("b", 64)
	if earlier == reg.Generation {
		earlier = strings.Repeat("c", 64)
	}
	return reg, earlier
}

// The roster reads a listed pane only for the record that registered it: a
// stale record is not observed through another agent's screen and fails as
// absent.
func TestRosterDoesNotReadAPaneWhoseIdAStaleRecordReuses(t *testing.T) {
	f := newStateFixture(t)
	reg, earlier := reusedPane(t, f)
	f.setAgent(t, f.add(t, "a", "live", "codex"), func(a *core.Agent) {
		a.Pane = reg.Pane
		a.Registration.Generation, a.Registration.Session = reg.Generation, reg.Session
	})
	// The stale record differs from the live one only by the server
	// generation that registered it.
	f.setAgent(t, f.add(t, "b", "stale", "codex"), func(a *core.Agent) {
		a.Pane = reg.Pane
		a.Registration.Generation, a.Registration.Session = earlier, reg.Session
	})
	rows := rosterRows(t, f)
	if row := rows["stale"]; !strings.Contains(row, " failed ") || !strings.HasSuffix(row, "registered pane is absent from tmux") {
		t.Fatalf("stale row = %q", row)
	}
	if row := rows["live"]; !strings.Contains(row, " active ") {
		t.Fatalf("live row = %q", row)
	}
	if f.input.captures != 1 {
		t.Fatalf("roster captured the pane %d times, want once for its registered record", f.input.captures)
	}
}

// A tick reads a pane only for the record that registered it, whether it
// probes an active agent or a startup past its deadline.
func TestTickDoesNotReadAPaneWhoseIdAStaleRecordReuses(t *testing.T) {
	for _, status := range []core.Status{core.Active, core.Booting} {
		t.Run(string(status), func(t *testing.T) {
			f := newStateFixture(t)
			reg, earlier := reusedPane(t, f)
			f.setAgent(t, f.add(t, "b", "stale", "codex"), func(a *core.Agent) {
				a.Pane, a.Status = reg.Pane, status
				a.Registration.Generation, a.Registration.Session = earlier, reg.Session
				if status == core.Booting {
					a.BootDeadline = f.cmd.now().Add(-time.Second)
				}
			})
			err := f.cmd.tick([]string{"--agent", "stale"})
			if err != nil && !errors.Is(err, tmux.ErrPaneReplaced) {
				t.Fatalf("tick: %v", err)
			}
			if f.input.captures != 0 {
				t.Fatalf("tick captured another record's pane %d times", f.input.captures)
			}
		})
	}
}
