package main

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/substrate"
	"github.com/adambiggs/gangline/substrate/tmux"
)

// A new tmux server numbers its panes from the start again, so a record from
// an ended server can name the id of another agent's live pane. The roster
// reads that pane only for the record that registered it: the stale record is
// not observed through the other agent's screen and fails as absent.
func TestRosterDoesNotReadAPaneWhoseIdAStaleRecordReuses(t *testing.T) {
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
	defer os.RemoveAll(root)
	socket := root + "/tmux.sock"
	f := newStateFixture(t)
	session := f.env["GANG_SESSION"]
	f.env["GANG_TMUX"], f.env["GANG_TMUX_SOCKET"] = binary, socket
	if out, err := exec.Command(binary, "-S", socket, "new-session", "-d", "-s", session, "cat").CombinedOutput(); err != nil {
		t.Fatalf("new-session: %v\n%s", err, out)
	}
	defer func() { _ = exec.Command(binary, "-S", socket, "kill-session", "-t", "="+session).Run() }()
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
	f.setAgent(t, f.add(t, "a", "live", "codex"), func(a *core.Agent) {
		a.Pane = string(pane.ID)
		a.Registration.Generation, a.Registration.Session = reg.Generation, reg.Session
	})
	// The stale record differs from the live one only by the server
	// generation that registered it.
	earlier := strings.Repeat("b", 64)
	if earlier == reg.Generation {
		earlier = strings.Repeat("c", 64)
	}
	f.setAgent(t, f.add(t, "b", "stale", "codex"), func(a *core.Agent) {
		a.Pane = string(pane.ID)
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
