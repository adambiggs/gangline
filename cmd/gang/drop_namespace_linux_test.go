//go:build linux

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/substrate"
	"github.com/adambiggs/gangline/substrate/tmux"
)

func TestDropMissingPaneFinishesRecordedTeardown(t *testing.T) {
	f := newStateFixture(t)
	child := exec.Command("cat")
	stdin, err := child.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = stdin.Close(); _ = child.Process.Kill(); _ = child.Wait() }()
	fake := f.env["GANG_TMUX"]
	script := fmt.Sprintf("#!/bin/sh\n# SPDX-License-Identifier: Apache-2.0\n"+fakeTmuxUTF8+"case \"$1\" in display-message) printf '%%s\\n' '%d 0';; *) printf 'no server running on fixture\\n' >&2; exit 1;; esac\n", child.Process.Pid)
	if err := os.WriteFile(fake, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	b, err := tmux.New(tmux.Config{Binary: fake, Session: "team"})
	if err != nil {
		t.Fatal(err)
	}
	identity, err := b.Identity(context.Background(), substrate.PaneID("%1"))
	if err != nil {
		t.Fatal(err)
	}
	f.cmd.paneBackend = b
	a := f.add(t, "a", "worker", "codex")
	a.Status = core.Failed
	a.Process = storedIdentity(identity)
	a.Teardown = []core.ProcessIdentity{a.Process}
	a.Registration = core.PaneRegistration{Generation: strings.Repeat("a", 64), Session: "$1", TokenHash: tokenHash("token")}
	p, _ := f.run.team.Agent(a.ID)
	l, err := p.LockAgent()
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Save(a); err != nil {
		t.Fatal(err)
	}
	l.Close()
	if err := f.cmd.drop([]string{"worker"}); err != nil {
		t.Fatal(err)
	}
	// Stop returns only after its pinned child has exited. WNOHANG makes a lost
	// teardown an immediate failure, without a polling loop or timeout.
	var status syscall.WaitStatus
	pid, err := syscall.Wait4(child.Process.Pid, &status, syscall.WNOHANG, nil)
	if err != nil || pid != child.Process.Pid {
		t.Fatalf("recorded child survived drop: pid=%d err=%v", pid, err)
	}
	if _, err := f.run.team.ResolveName("worker"); err == nil {
		t.Fatal("failed registration remains")
	}
}

// An exited original root yields no teardown targets. The final pane removal
// must still reject a replacement that kept the same tmux pane identifier.
type exitedRootRegistry struct{ *tmux.Backend }

func (*exitedRootRegistry) AcquireTree(context.Context, substrate.PaneID, tmux.Identity) (*tmux.Owned, error) {
	return &tmux.Owned{}, nil
}

func TestDropRegisteredRespawnKeepsReplacementAndRegistration(t *testing.T) {
	t.Setenv("TMUX", "")
	t.Setenv("TMUX_PANE", "")
	binary, err := exec.LookPath("tmux")
	if err != nil {
		t.Skip("tmux is required")
	}
	root, err := os.MkdirTemp("", "gang-drop-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(root)
	socket := root + "/tmux.sock"
	const session = "registered-drop-test"
	runTmux := func(args ...string) string {
		t.Helper()
		out, err := exec.Command(binary, append([]string{"-S", socket}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("tmux %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	runTmux("new-session", "-d", "-s", session, "cat")
	defer runTmux("kill-session", "-t", "="+session)
	if got := runTmux("list-sessions", "-F", "#{session_name}"); got != session {
		t.Fatalf("sessions = %q", got)
	}
	b, err := tmux.New(tmux.Config{Binary: binary, Socket: socket, Session: session})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	pane, err := b.Spawn(ctx, substrate.SpawnSpec{Name: "worker", Directory: root, Command: "cat"})
	if err != nil {
		t.Fatal(err)
	}
	reg, err := b.RegisterPane(ctx, pane.ID)
	if err != nil {
		t.Fatal(err)
	}
	old, err := b.Identity(ctx, pane.ID)
	if err != nil {
		t.Fatal(err)
	}
	runTmux("respawn-pane", "-k", "-t", string(pane.ID), "cat")
	replacement, err := b.Identity(ctx, pane.ID)
	if err != nil || replacement.PID == old.PID {
		t.Fatalf("replacement = %+v, %v; old %+v", replacement, err, old)
	}
	f := newStateFixture(t)
	f.cmd.paneBackend = &exitedRootRegistry{b}
	a := f.add(t, "a", "worker", "codex")
	a.Status = core.Failed
	a.Pane = string(pane.ID)
	a.Process = storedIdentity(old)
	a.Registration = core.PaneRegistration{Generation: reg.Generation, Session: reg.Session, TokenHash: tokenHash("token")}
	p, _ := f.run.team.Agent(a.ID)
	l, err := p.LockAgent()
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Save(a); err != nil {
		t.Fatal(err)
	}
	l.Close()
	if err := f.cmd.drop([]string{"worker"}); err == nil || !strings.Contains(err.Error(), "identity changed") {
		t.Fatalf("drop replacement: %v", err)
	}
	if got, err := b.Identity(ctx, pane.ID); err != nil || got != replacement {
		t.Fatalf("replacement changed: %+v, %v", got, err)
	}
	if _, err := f.run.team.ResolveName("worker"); err != nil {
		t.Fatalf("registration lost: %v", err)
	}
}
