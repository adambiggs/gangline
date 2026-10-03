//go:build linux

package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/substrate"
	"github.com/adambiggs/gangline/substrate/tmux"
)

// A team recorded under an earlier boot ended with it: its tmux server and
// every recorded process are gone, and a recorded PID now names whatever holds
// it on this boot. The roster says the team did not survive the reboot, and
// drop and down clear its records without reading or signalling those PIDs.
func TestDropAndDownClearATeamFromAnEarlierBoot(t *testing.T) {
	t.Setenv("TMUX", "")
	t.Setenv("TMUX_PANE", "")
	binary, err := exec.LookPath("tmux")
	if err != nil {
		t.Skip("tmux is required")
	}
	root, err := os.MkdirTemp("", "gang-earlierboot-")
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
	// A process of a second server, which the reboot leaves running, holds
	// the recorded PID under the recorded start time: only the boot differs.
	holderSession := session + "-holder"
	if out, err := exec.Command(binary, "-S", socket+"-holder", "new-session", "-d", "-s", holderSession, "cat").CombinedOutput(); err != nil {
		t.Fatalf("holder session: %v\n%s", err, out)
	}
	defer func() {
		_ = exec.Command(binary, "-S", socket+"-holder", "kill-session", "-t", "="+holderSession).Run()
	}()
	holderBackend, err := tmux.New(tmux.Config{Binary: binary, Socket: socket + "-holder", Session: holderSession})
	if err != nil {
		t.Fatal(err)
	}
	windows, err := holderBackend.Windows(ctx)
	if err != nil || len(windows) != 1 {
		t.Fatalf("holder windows = %v, %v", windows, err)
	}
	holder, err := holderBackend.Identity(ctx, windows[0].Pane.ID)
	if err != nil {
		t.Fatal(err)
	}
	names := []string{"first", "second", "third"}
	for i, name := range names {
		pane, err := b.Spawn(ctx, substrate.SpawnSpec{Name: name, Directory: root, Command: "cat"})
		if err != nil {
			t.Fatal(err)
		}
		reg, err := b.RegisterPane(ctx, pane.ID)
		if err != nil {
			t.Fatal(err)
		}
		recorded := storedIdentity(holder)
		recorded.BootID = "earlier-boot"
		f.setAgent(t, f.add(t, string(rune('a'+i)), name, "codex"), func(a *core.Agent) {
			a.Pane = string(pane.ID)
			a.Process = recorded
			a.Registration.Generation, a.Registration.Session = reg.Generation, reg.Session
		})
	}
	// The reboot ends the team's server with its last session.
	if out, err := exec.Command(binary, "-S", socket, "kill-session", "-t", "="+session).CombinedOutput(); err != nil {
		t.Fatalf("kill-session: %v\n%s", err, out)
	}
	if out, err := exec.Command(binary, "-S", socket, "list-sessions").CombinedOutput(); err == nil {
		t.Fatalf("team server still runs:\n%s", out)
	}
	if err := f.cmd.drop([]string{names[0]}); err != nil {
		t.Fatalf("drop: %v", err)
	}
	if _, err := f.run.team.ResolveName(names[0]); err == nil {
		t.Fatal("dropped agent remains registered")
	}
	rows := rosterRows(t, f)
	for _, name := range names[1:] {
		if row := rows[name]; !strings.Contains(row, " failed ") || !strings.HasSuffix(row, "the team did not survive a host reboot; gang down clears its records") {
			t.Fatalf("%s row = %q", name, row)
		}
	}
	if err := f.cmd.down([]string{"--yes"}); err != nil {
		t.Fatalf("down: %v", err)
	}
	if _, err := os.Stat(f.run.team.Directory); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("team remains: %v", err)
	}
	if again, err := holderBackend.Identity(ctx, windows[0].Pane.ID); err != nil || again != holder {
		t.Fatalf("process holding the recorded PID changed: %+v, %v", again, err)
	}
}
