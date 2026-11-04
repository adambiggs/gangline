//go:build acceptance

package main

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHitchRefusesUnidentifiedOperatorPane(t *testing.T) {
	for _, command := range []string{"zsh", "vim"} {
		t.Run(command, func(t *testing.T) {
			f := newStateFixture(t)
			fakeCodexOnPath(t)
			// The listener establishes that tmux's reported server PID is in
			// this namespace. The test process supplies a real non-agent tree.
			// macOS's temporary root plus t.TempDir's full test name can
			// exceed the Unix socket address limit. Keep this component short.
			root, err := os.MkdirTemp(os.TempDir(), "pane-")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { os.RemoveAll(root) })
			socket := filepath.Join(root, "s")
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
			// Even when tmux reports a shell, this tree contains the test
			// executable; it cannot exclude an unknown background harness.
			if err == nil || !strings.Contains(err.Error(), "unregistered pane") {
				t.Fatalf("hitch error = %v, want unidentified pane refusal", err)
			}
		})
	}
}
