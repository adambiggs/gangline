package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adambiggs/gangline/substrate"
)

func TestHitchAllowsUnregisteredOperatorPane(t *testing.T) {
	for _, command := range []string{"zsh", "vim"} {
		t.Run(command, func(t *testing.T) {
			f := newStateFixture(t)
			fakeCodexOnPath(t)
			// The listener establishes that tmux's reported server PID is in
			// this namespace. The test process supplies a real non-agent tree.
			socket := filepath.Join(t.TempDir(), "s")
			listener, err := net.Listen("unix", socket)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { listener.Close() })
			script := fmt.Sprintf(`#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
%s
case "$1" in
list-panes) printf '%%%%1\t%s\t$1\t-lead-\n';;
has-session) exit 0;;
display-message) case "$5" in
'#{pane_current_command}') printf '%s\n';;
'#{socket_path}\t#{pid}\t#{pane_id}') printf '%s\t%d\t%%%%1\n';;
*) printf '%d 0\n';;
esac;;
*) exit 91;;
esac
`, fakeTmuxUTF8, strings.Repeat("a", 64), command, socket, os.Getpid(), os.Getpid())
			if err := os.WriteFile(f.env["GANG_TMUX"], []byte(script), 0700); err != nil {
				t.Fatal(err)
			}
			err = f.cmd.hitch([]string{"worker", "-c", "codex"})
			// Creation is deliberately refused by the fixture. Getting there
			// establishes that the operator pane did not refuse the hitch.
			if err == nil || !strings.Contains(err.Error(), "spawn pane") {
				t.Fatalf("hitch error = %v, want spawn pane", err)
			}
		})
	}
}

type unregisteredFixture struct {
	command   string
	visible   bool
	processes []substrate.Process
	err       error
}

func (f unregisteredFixture) ForegroundCommand(context.Context, substrate.PaneID) (string, error) {
	return f.command, nil
}
func (f unregisteredFixture) ProcessVisibility(context.Context, substrate.PaneID) (bool, error) {
	return f.visible, nil
}
func (f unregisteredFixture) PaneProcesses(context.Context, substrate.PaneID) ([]substrate.Process, error) {
	return f.processes, f.err
}

func TestUnregisteredAgentChecksBackgroundAndInvokedName(t *testing.T) {
	commands := map[string]bool{"codex": true, "claude": true, "custom": true}
	for _, tc := range []struct {
		name string
		f    unregisteredFixture
		want bool
	}{
		{"shell", unregisteredFixture{command: "zsh", visible: true}, false},
		{"foreground", unregisteredFixture{command: "codex"}, true},
		{"background", unregisteredFixture{command: "zsh", visible: true, processes: []substrate.Process{{PID: 3, GroupID: 3, Command: "/bin/codex"}}}, true},
		{"symlink", unregisteredFixture{command: "version", visible: true, processes: []substrate.Process{{Command: "/bin/claude", Name: "version"}}}, true},
		{"custom", unregisteredFixture{command: "custom"}, true},
		{"private namespace", unregisteredFixture{command: "zsh", err: errors.New("must not read")}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := unregisteredAgent(context.Background(), tc.f, "%1", commands)
			if err != nil || got != tc.want {
				t.Fatalf("running=%v err=%v, want %v", got, err, tc.want)
			}
		})
	}
	_, err := unregisteredAgent(context.Background(), unregisteredFixture{command: "zsh", visible: true, err: errors.New("tree unavailable")}, "%1", commands)
	if err == nil || !strings.Contains(err.Error(), "tree unavailable") {
		t.Fatalf("unknown process tree accepted: %v", err)
	}
}
