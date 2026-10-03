package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adambiggs/gangline/core"
)

// A record that names a pane without a complete registration cannot prove the
// pane is gang's, so a roster read lists it and leaves its window title alone.
func TestRosterLeavesWindowOfUnregisteredPaneUnmarked(t *testing.T) {
	f := newStateFixture(t)
	f.cmd.inputBackend = nil
	calls := filepath.Join(t.TempDir(), "calls")
	script := "#!/bin/sh\n# SPDX-License-Identifier: Apache-2.0\n" + fakeTmuxUTF8 + "printf '%s\\n' \"$*\" >>'" + calls + "'\ncase \"$1\" in list-panes) printf '%%1\\tworker\\n';; has-session) exit 0;; *) exit 91;; esac\n"
	if err := os.WriteFile(f.env["GANG_TMUX"], []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	f.setAgent(t, f.add(t, "a", "worker", "codex"), func(a *core.Agent) {
		a.Status, a.Activity, a.Evidence = core.Failed, core.Unknown, "boot deadline elapsed"
		a.Registration = core.PaneRegistration{}
	})
	row := rosterRows(t, f)["worker"]
	if !strings.Contains(row, "failed") || !strings.Contains(row, "boot deadline elapsed") {
		t.Fatalf("row = %q", row)
	}
	data, err := os.ReadFile(calls)
	if err != nil {
		t.Fatal(err)
	}
	// The listing shows the title differs from the record's, so only the
	// registration check keeps the rename back.
	if !strings.Contains(string(data), "list-panes") {
		t.Fatalf("roster did not list panes: %q", data)
	}
	for _, call := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if !strings.HasPrefix(call, "list-panes -s ") && !strings.HasPrefix(call, "has-session") {
			t.Fatalf("roster ran tmux %q for an unregistered pane", call)
		}
	}
}
