package main

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/substrate"
	"github.com/adambiggs/gangline/substrate/tmux"
)

// paneProcesses returns the identity of a process running in a pane on a
// private tmux server that the test ends, and the identity of a process that
// ran in a second pane on that server and has exited.
func paneProcesses(t *testing.T) (live, gone core.ProcessIdentity) {
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
	return storedIdentity(identity), exitedPaneProcess(t, b, binary, socket, root)
}

// exitedPaneProcess records a pane process, releases it, and returns once the
// server has collected it. The process holds the write end of an exit pipe,
// so EOF on the pipe witnesses its exit; the reap makes the server collect
// every exited child before it returns.
func exitedPaneProcess(t *testing.T, b *tmux.Backend, binary, socket, root string) core.ProcessIdentity {
	t.Helper()
	fifo := filepath.Join(root, "exit-pipe")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	pane, err := b.Spawn(ctx, substrate.SpawnSpec{
		Name: "gone", Directory: root, Command: "sh",
		Args: []string{"-c", `exec 3>"$3"; "$1" -S "$2" wait-for release`, "sh", binary, socket, fifo},
	})
	if err != nil {
		t.Fatal(err)
	}
	opened := make(chan *os.File, 1)
	go func() {
		// Opening blocks until the process holds the write end.
		if pipe, err := os.Open(fifo); err == nil {
			opened <- pipe
		}
	}()
	var pipe *os.File
	// Bounded so a process that never starts cannot hold the test forever.
	select {
	case pipe = <-opened:
		t.Cleanup(func() { pipe.Close() })
	case <-time.After(time.Minute):
		t.Fatal("process did not start")
	}
	identity, err := b.Identity(ctx, pane.ID)
	if err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(binary, "-S", socket, "wait-for", "-S", "release").CombinedOutput(); err != nil {
		t.Fatalf("release: %v\n%s", err, out)
	}
	exited := make(chan error, 1)
	go func() {
		_, err := io.Copy(io.Discard, pipe)
		exited <- err
	}()
	// Bounded so a process that never exits cannot hold the test forever.
	select {
	case err := <-exited:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Minute):
		t.Fatal("process did not exit")
	}
	// Bounded so a server that stops answering cannot hold the test forever.
	reap, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	if out, err := tmux.Reaped(reap, func(ctx context.Context, arguments ...string) (string, error) {
		out, err := exec.CommandContext(ctx, binary, append([]string{"-S", socket}, arguments...)...).CombinedOutput()
		return string(out), err
	}); err != nil {
		t.Fatalf("reap: %v\n%s", err, out)
	}
	return storedIdentity(identity)
}

// A socket that answers nothing lists no panes, which says nothing of a pane
// the registering server may still run. The roster reports an agent whose
// recorded process runs as unknown and leaves its record as it was; an agent
// whose recorded process has exited fails, whether the machine rebooted or the
// process ended or was replaced on this boot.
func TestRosterWithoutTeamListingKeepsRecordOfLiveProcess(t *testing.T) {
	live, gone := paneProcesses(t)
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
	// A replacement holds the recorded PID under the identity the platform
	// witnesses: a unique id where it has one, otherwise the start time.
	reused := live
	if reused.UniqueID != 0 {
		reused.UniqueID++
	} else {
		reused.Started += "-replaced"
	}
	for i, process := range []core.ProcessIdentity{gone, reused} {
		name := []string{"gone", "reused"}[i]
		f.setAgent(t, f.add(t, string(rune('g'+i)), name, "codex"), func(a *core.Agent) { a.Process = process })
	}
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
		for _, name := range []string{"rebooted", "gone", "reused"} {
			if row := rows[name]; !strings.Contains(row, " failed ") || !strings.HasSuffix(row, "tmux lists no team session and the recorded process has exited") {
				t.Fatalf("%s row = %q", name, row)
			}
		}
		if row := rows["failed"]; !strings.HasSuffix(row, "boot deadline elapsed") {
			t.Fatalf("failed row = %q", row)
		}
	}
	got := f.agent(t, a.ID)
	if got.Status != core.Active || got.Activity != core.Idle || got.Pane != a.Pane || got.Evidence != "" {
		t.Fatalf("record changed: status=%s activity=%s pane=%q evidence=%q", got.Status, got.Activity, got.Pane, got.Evidence)
	}
	if n := eventsOfType(t, f, "hitch_failed"); n != 3 {
		t.Fatalf("hitch_failed events = %d, want 3", n)
	}
}
