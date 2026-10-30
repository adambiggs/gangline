//go:build acceptance

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseHitchWindow(t *testing.T) {
	got, err := parseHitch([]string{"worker", "--window", "7"}, "claude", "/work")
	if err != nil || !got.Window.set || got.Window.index != 7 {
		t.Fatalf("parse --window 7 = %+v, %v", got.Window, err)
	}
	if got, err := parseHitch([]string{"worker"}, "claude", "/work"); err != nil || got.Window.set {
		t.Fatalf("parse without --window = %+v, %v", got.Window, err)
	}
	for _, args := range [][]string{
		{"worker", "--window", "-1"},
		{"worker", "--window", "x"},
		{"worker", "--window", "99999999999"},
		{"worker", "--window", ""},
		{"worker", "--window", "1", "--split", "lead"},
		{"worker", "--window", "1", "--recover"},
	} {
		if _, err := parseHitch(args, "claude", "/work"); err == nil {
			t.Errorf("parseHitch(%q) accepted", args)
		}
	}
	for _, option := range optionsFor("up") {
		if option.name == "window" {
			t.Fatal("up offers --window, but tmux places a new session's first window")
		}
	}
}

// windowHitch runs a hitch with --window against a tmux that lists one window
// at index 3 and records the arguments of any new-window.
func windowHitch(t *testing.T, sessionExists bool, args ...string) (error, string, *stateFixture) {
	t.Helper()
	f := newStateFixture(t)
	fakeCodexOnPath(t)
	capture := filepath.Join(t.TempDir(), "launch")
	t.Setenv("GANGLINE_TEST_LAUNCH_CAPTURE", capture)
	session := "exit 0"
	if !sessionExists {
		session = "exit 1"
	}
	script := "#!/bin/sh\n# SPDX-License-Identifier: Apache-2.0\n" + fakeTmuxUTF8 + `
case "$1" in
has-session) ` + session + ` ;;
list-panes) exit 0 ;;
list-windows) printf '3\t@1\tlead\n' ;;
new-window) printf '%s\n' "$@" > "$GANGLINE_TEST_LAUNCH_CAPTURE"; exit 91 ;;
*) exit 92 ;;
esac
`
	if err := os.WriteFile(f.env["GANG_TMUX"], []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	err := f.cmd.hitch(append([]string{"owner", "-c", "codex", "-d", t.TempDir()}, args...))
	launch, _ := os.ReadFile(capture)
	return err, string(launch), f
}

func TestHitchWindowTargetsItsIndex(t *testing.T) {
	err, launch, _ := windowHitch(t, true, "--window", "7")
	// Stop at allocation, after the hitch has chosen the window.
	if err == nil || !strings.Contains(err.Error(), "spawn pane") {
		t.Fatalf("hitch did not reach the launch barrier: %v", err)
	}
	if !strings.Contains(launch, "\n-t\n=unit:7\n") || strings.Contains(launch, "\n-b\n") {
		t.Fatalf("new-window does not target index 7: %s", launch)
	}
}

func TestHitchWindowRefusedBeforeClaim(t *testing.T) {
	for _, c := range []struct {
		exists bool
		want   string
	}{
		{true, `window index 3 in use by window "lead" in team "unit"`},
		{false, `team "unit" has no tmux session for --window`},
	} {
		err, launch, f := windowHitch(t, c.exists, "--window", "3")
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Fatalf("hitch --window 3: %v; want %q", err, c.want)
		}
		if launch != "" {
			t.Fatalf("refused hitch created a window: %s", launch)
		}
		if id, err := f.run.team.ResolveName("owner"); err == nil {
			t.Fatalf("refused hitch claimed the name as %s", id)
		}
	}
}
