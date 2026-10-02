//go:build linux

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/substrate"
	"github.com/adambiggs/gangline/substrate/tmux"
)

// A tmux server exits when its last pane closes, and nothing then answers on
// its socket. The record's witness of that server's process tells its exit
// from a socket gang cannot reach: tick and roster fail the agent and forget
// the pane only when the process is gone.
func TestPaneOfExitedServerIsForgotten(t *testing.T) {
	f := newStateFixture(t)
	f.cmd.inputBackend, f.cmd.paneBackend = nil, nil
	fake := f.env["GANG_TMUX"]
	// The fixture answers a pane process read with the given PID, which is how
	// a process identity is read here, and has no server for anything else.
	absent := "printf 'no server running on fixture\\n' >&2; exit 1"
	identity := func(pid int) core.ProcessIdentity {
		t.Helper()
		script := fmt.Sprintf("#!/bin/sh\n# SPDX-License-Identifier: Apache-2.0\ncase \"$1\" in display-message) printf '%%s\\n' '%d 0';; *) %s;; esac\n", pid, absent)
		if err := os.WriteFile(fake, []byte(script), 0700); err != nil {
			t.Fatal(err)
		}
		b, err := tmux.New(tmux.Config{Binary: fake, Session: "team"})
		if err != nil {
			t.Fatal(err)
		}
		id, err := b.Identity(context.Background(), substrate.PaneID("%1"))
		if err != nil {
			t.Fatal(err)
		}
		return storedIdentity(id)
	}
	child := exec.Command("cat")
	stdin, err := child.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	exited, running := identity(child.Process.Pid), identity(os.Getpid())
	// Wait reaps the child, so its process is gone when it returns.
	_ = stdin.Close()
	if err := child.Wait(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fake, []byte("#!/bin/sh\n# SPDX-License-Identifier: Apache-2.0\n"+absent+"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	witnessed := func(id, name string, server core.ProcessIdentity) core.Agent {
		return f.setAgent(t, f.add(t, id, name, "codex"), func(a *core.Agent) { a.Registration.Server = server })
	}
	ticked, listed, kept := witnessed("a", "ticked", exited), witnessed("b", "listed", exited), witnessed("c", "kept", running)
	if err := f.cmd.tick([]string{"--agent", "ticked"}); err != nil {
		t.Fatalf("tick of an agent whose server exited: %v", err)
	}
	if err := f.cmd.tick([]string{"--agent", "kept"}); err == nil || !strings.Contains(err.Error(), "no server running") {
		t.Fatalf("tick of an agent whose server still runs: %v", err)
	}
	rows := rosterRows(t, f)
	for _, a := range []core.Agent{ticked, listed} {
		got := f.agent(t, a.ID)
		if got.Status != core.Failed || got.Pane != "" || got.Evidence != "registered pane is absent from tmux" {
			t.Fatalf("%s after its server exited: status=%s pane=%q evidence=%q", a.Name, got.Status, got.Pane, got.Evidence)
		}
		if !strings.Contains(rows[string(a.Name)], "registered pane is absent from tmux") {
			t.Fatalf("%s row = %q", a.Name, rows[string(a.Name)])
		}
	}
	if got := f.agent(t, kept.ID); got.Pane != "%1" {
		t.Fatalf("agent whose server still runs lost its pane: status=%s pane=%q", got.Status, got.Pane)
	}
}
