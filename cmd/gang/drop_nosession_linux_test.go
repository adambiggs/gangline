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

// A server whose last session ended can keep running and answer every pane
// read with "no current target". Drop and down read its registered panes as
// gone and finish the recorded teardown.
func TestDropAndDownOnServerWithNoSession(t *testing.T) {
	t.Setenv("TMUX", "")
	t.Setenv("TMUX_PANE", "")
	binary, err := exec.LookPath("tmux")
	if err != nil {
		t.Skip("tmux is required")
	}
	root, err := os.MkdirTemp("", "gang-nosession-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(root)
	socket := root + "/tmux.sock"
	runTmux := func(args ...string) string {
		t.Helper()
		out, err := exec.Command(binary, append([]string{"-S", socket}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("tmux %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	f := newStateFixture(t)
	session := f.env["GANG_SESSION"]
	f.env["GANG_TMUX"], f.env["GANG_TMUX_SOCKET"] = binary, socket
	runTmux("new-session", "-d", "-s", session, "cat")
	defer func() {
		_ = exec.Command(binary, "-S", socket, "kill-session", "-t", "="+session).Run()
		_ = exec.Command(binary, "-S", socket, "set-option", "-s", "exit-empty", "on").Run()
	}()
	b, err := tmux.New(tmux.Config{Binary: binary, Socket: socket, Session: session})
	if err != nil {
		t.Fatal(err)
	}
	f.cmd.paneBackend = b
	ctx := context.Background()
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
		identity, err := b.Identity(ctx, pane.ID)
		if err != nil {
			t.Fatal(err)
		}
		f.setAgent(t, f.add(t, string(rune('a'+i)), name, "codex"), func(a *core.Agent) {
			a.Pane = string(pane.ID)
			a.Process = storedIdentity(identity)
			a.Registration.Generation, a.Registration.Session = reg.Generation, reg.Session
		})
	}
	runTmux("set-option", "-s", "exit-empty", "off")
	runTmux("kill-session", "-t", "="+session)
	if out, err := exec.Command(binary, "-S", socket, "list-panes", "-a").CombinedOutput(); err == nil || !strings.HasPrefix(string(out), "no current target") {
		t.Fatalf("server did not stay up with no session: %v\n%s", err, out)
	}
	if err := f.cmd.drop([]string{names[0]}); err != nil {
		t.Fatalf("drop: %v", err)
	}
	if _, err := f.run.team.ResolveName(names[0]); err == nil {
		t.Fatal("dropped agent remains registered")
	}
	if err := f.cmd.down([]string{"--yes"}); err != nil {
		t.Fatalf("down: %v", err)
	}
	if _, err := os.Stat(f.run.team.Directory); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("team remains: %v", err)
	}
}
