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

// livePaneProcess returns the identity of a process running in a pane on a
// private tmux server that the test ends.
func livePaneProcess(t *testing.T) core.ProcessIdentity {
	t.Helper()
	t.Setenv("TMUX", "")
	t.Setenv("TMUX_PANE", "")
	binary, err := exec.LookPath("tmux")
	if err != nil {
		t.Skip("tmux is required")
	}
	root, err := os.MkdirTemp("", "gang-nolisting-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	socket, session := root+"/tmux.sock", "nolisting"
	if out, err := exec.Command(binary, "-S", socket, "new-session", "-d", "-s", session, "cat").CombinedOutput(); err != nil {
		t.Fatalf("new-session: %v\n%s", err, out)
	}
	t.Cleanup(func() { _ = exec.Command(binary, "-S", socket, "kill-session", "-t", "="+session).Run() })
	b, err := tmux.New(tmux.Config{Binary: binary, Socket: socket, Session: session})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	pane, err := b.Spawn(ctx, substrate.SpawnSpec{Name: "live", Directory: root, Command: "cat"})
	if err != nil {
		t.Fatal(err)
	}
	identity, err := b.Identity(ctx, pane.ID)
	if err != nil {
		t.Fatal(err)
	}
	return storedIdentity(identity)
}

// A socket that answers nothing lists no panes, which says nothing of a pane
// the registering server may still run. The roster reports an agent whose
// recorded process runs as unknown and leaves its record as it was; an agent
// whose recorded process has exited fails.
func TestRosterWithoutTeamListingKeepsRecordOfLiveProcess(t *testing.T) {
	live := livePaneProcess(t)
	f := newStateFixture(t)
	program := "#!/bin/sh\n# SPDX-License-Identifier: Apache-2.0\n" +
		"printf 'no server running on fixture\\n' >&2; exit 1\n"
	if err := os.WriteFile(f.env["GANG_TMUX"], []byte(program), 0700); err != nil {
		t.Fatal(err)
	}
	a := f.setAgent(t, f.add(t, "a", "worker", "codex"), func(a *core.Agent) { a.Process = live })
	// Identities gang cannot check witness nothing either way.
	unverified, unrecorded, foreign := live, live, live
	unverified.BootID, unrecorded.PID, foreign.Namespace = "", 0, "other-namespace"
	for i, process := range []core.ProcessIdentity{unverified, unrecorded, foreign} {
		name := []string{"unverified", "unrecorded", "foreign"}[i]
		f.setAgent(t, f.add(t, string(rune('d'+i)), name, "codex"), func(a *core.Agent) { a.Process = process })
	}
	rebooted := live
	rebooted.BootID = "earlier-boot"
	f.setAgent(t, f.add(t, "b", "rebooted", "codex"), func(b *core.Agent) { b.Process = rebooted })
	f.setAgent(t, f.add(t, "c", "failed", "codex"), func(c *core.Agent) {
		c.Status, c.Activity, c.Evidence = core.Failed, core.Unknown, "boot deadline elapsed"
	})
	for range 2 {
		rows := rosterRows(t, f)
		for _, name := range []string{"worker", "unverified", "unrecorded", "foreign"} {
			if row := rows[name]; !strings.Contains(row, " active ") || !strings.Contains(row, " unknown ") || !strings.HasSuffix(row, "tmux lists no team session") {
				t.Fatalf("%s row = %q", name, row)
			}
		}
		if row := rows["rebooted"]; !strings.Contains(row, " failed ") || !strings.HasSuffix(row, "tmux lists no team session and the recorded process has exited") {
			t.Fatalf("rebooted row = %q", row)
		}
		if row := rows["failed"]; !strings.HasSuffix(row, "boot deadline elapsed") {
			t.Fatalf("failed row = %q", row)
		}
	}
	got := f.agent(t, a.ID)
	if got.Status != core.Active || got.Activity != core.Idle || got.Pane != a.Pane || got.Evidence != "" {
		t.Fatalf("record changed: status=%s activity=%s pane=%q evidence=%q", got.Status, got.Activity, got.Pane, got.Evidence)
	}
	if n := eventsOfType(t, f, "hitch_failed"); n != 1 {
		t.Fatalf("hitch_failed events = %d, want 1", n)
	}
}
