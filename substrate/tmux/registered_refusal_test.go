package tmux

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adambiggs/gangline/substrate"
)

// A refusal describes the registered pane, not the pane tmux would pick when a
// command names no target.
func TestRegisteredInputRefusalDescribesRegisteredPane(t *testing.T) {
	t.Setenv("TMUX", "")
	t.Setenv("TMUX_PANE", "")
	binary, err := exec.LookPath("tmux")
	if err != nil {
		t.Skip("tmux is required")
	}
	root := privateTmuxRoot(t)
	socket := filepath.Join(root, "tmux.sock")
	const session = "refusal-pane-test"
	runTmux(t, binary, socket, "new-session", "-d", "-s", session, "cat")
	t.Cleanup(func() { runTmux(t, binary, socket, "kill-session", "-t", "="+session) })
	b, err := New(Config{Binary: binary, Socket: socket, Session: session})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	pane, err := b.Spawn(ctx, substrate.SpawnSpec{Name: "registered", Directory: root, Command: "sh", Args: []string{"-c", `"$1" -S "$2" wait-for -S ready; read line`, "sh", binary, socket}})
	if err != nil {
		t.Fatal(err)
	}
	id, err := b.RegisterPane(ctx, pane.ID)
	if err != nil {
		t.Fatal(err)
	}
	// The shell stays the foreground process once it has signalled.
	runTmux(t, binary, socket, "wait-for", "ready")
	foreground, err := b.ForegroundCommand(ctx, pane.ID)
	if err != nil || foreground == "cat" {
		t.Fatalf("registered pane foreground = %q: %v", foreground, err)
	}
	defaultPane := strings.TrimSpace(runTmux(t, binary, socket, "display-message", "-p", "#{pane_id}"))
	if defaultPane == id.Pane {
		t.Fatalf("fixture: tmux's default target is the registered pane %s", id.Pane)
	}
	t.Logf("panes:\n%s", runTmux(t, binary, socket, "list-panes", "-s", "-F", "#{pane_id} window=#{window_id} cmd=#{pane_current_command}"))
	for _, tc := range []struct{ name, text string }{
		{"inline", "x"},
		{"source-file", strings.Repeat("x", maxInlineRegisteredCommandBytes+1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := b.SendRegisteredKeys(ctx, id, "not-the-foreground", substrate.Keys{Text: tc.text})
			if err == nil {
				t.Fatal("input with a wrong foreground name was accepted")
			}
			t.Logf("refusal: %v", err)
			for _, want := range []string{"pane=" + id.Pane + ",", "session=" + id.Session + ",", "foreground=" + foreground + ","} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("refusal lacks %q: %v", want, err)
				}
			}
			if strings.Contains(err.Error(), "pane="+defaultPane+",") || strings.Contains(err.Error(), "foreground=cat") {
				t.Errorf("refusal describes the default pane %s: %v", defaultPane, err)
			}
		})
	}
}
