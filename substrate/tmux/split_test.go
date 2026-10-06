package tmux

import (
	"context"
	"github.com/adambiggs/gangline/substrate"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestSplitPreservesLinkedAndGroupedRegistrations(t *testing.T) {
	t.Setenv("TMUX", "")
	t.Setenv("TMUX_PANE", "")
	binary, err := exec.LookPath("tmux")
	if err != nil {
		t.Fatal(err)
	}
	for _, view := range []string{"linked", "grouped"} {
		t.Run(view, func(t *testing.T) {
			root := privateTmuxRoot(t)
			socket := filepath.Join(root, "tmux.sock")
			session := "split-test-" + view
			runTmux(t, binary, socket, "new-session", "-d", "-s", session)
			t.Cleanup(func() {
				runTmux(t, binary, socket, "kill-session", "-t", "="+session+"-view")
				runTmux(t, binary, socket, "kill-session", "-t", "="+session)
			})
			b, _ := New(Config{Binary: binary, Socket: socket, Session: session})
			ctx := context.Background()
			target, err := b.Spawn(ctx, substrate.SpawnSpec{Name: "target", Directory: root, Command: "cat"})
			if err != nil {
				t.Fatal(err)
			}
			id, err := b.RegisterPane(ctx, target.ID)
			if err != nil {
				t.Fatal(err)
			}
			if view == "linked" {
				runTmux(t, binary, socket, "new-session", "-d", "-s", session+"-view")
				runTmux(t, binary, socket, "link-window", "-s", id.Pane, "-t", "="+session+"-view:")
			} else {
				runTmux(t, binary, socket, "new-session", "-d", "-t", "="+session, "-s", session+"-view")
			}
			if got := strings.TrimSpace(runTmux(t, binary, socket, "display-message", "-p", "-t", id.Pane, "#{session_id}")); got == id.Session {
				t.Fatalf("wrong session view setup")
			}
			pane, err := b.Split(ctx, id, substrate.SpawnSpec{Name: "sibling", Directory: root, Command: "cat", KeepExited: true, Env: map[string]string{"SPLIT_LITERAL": "quote \" dollar $9; slash \\ newline\nend"}}, false)
			if err != nil {
				t.Fatal(err)
			}
			sibling, err := b.RegisterPane(ctx, pane.ID)
			if err != nil {
				t.Fatal(err)
			}
			if sibling.Session != id.Session || sibling.Generation != id.Generation {
				t.Fatalf("split registration differs: %+v %+v", id, sibling)
			}
			if got := runTmux(t, binary, socket, "display-message", "-p", "-t", id.Pane, "#{window_id}"); got != runTmux(t, binary, socket, "display-message", "-p", "-t", sibling.Pane, "#{window_id}") {
				t.Fatal("split created another window")
			}
		})
	}
}

func TestSplitRefusesTargetMovedAfterCheck(t *testing.T) {
	t.Setenv("TMUX", "")
	t.Setenv("TMUX_PANE", "")
	binary, err := exec.LookPath("tmux")
	if err != nil {
		t.Fatal(err)
	}
	root := privateTmuxRoot(t)
	socket := filepath.Join(root, "tmux.sock")
	session := "split-test-race"
	runTmux(t, binary, socket, "new-session", "-d", "-s", session)
	runTmux(t, binary, socket, "new-session", "-d", "-s", session+"-view")
	t.Cleanup(func() {
		runTmux(t, binary, socket, "kill-session", "-t", "="+session+"-view")
		runTmux(t, binary, socket, "kill-session", "-t", "="+session)
	})
	b, _ := New(Config{Binary: binary, Socket: socket, Session: session})
	ctx := context.Background()
	pane, err := b.Spawn(ctx, substrate.SpawnSpec{Name: "target", Directory: root, Command: "cat"})
	if err != nil {
		t.Fatal(err)
	}
	id, err := b.RegisterPane(ctx, pane.ID)
	if err != nil {
		t.Fatal(err)
	}
	wrapper := filepath.Join(root, "tmux-wrapper")
	script := "#!/bin/sh\n# SPDX-License-Identifier: Apache-2.0\ncase \"$*\" in *'source-file -') '" + binary + "' -S '" + socket + "' move-window -s '" + id.Pane + "' -t '=" + session + "-view:';; esac\nexec '" + binary + "' \"$@\"\n"
	if err := os.WriteFile(wrapper, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	b.config.Binary = wrapper
	before := runTmux(t, binary, socket, "list-panes", "-a", "-F", "#{pane_id}")
	_, err = b.Split(ctx, id, substrate.SpawnSpec{Name: "sibling", Directory: root, Command: "cat"}, false)
	if err == nil || !strings.Contains(err.Error(), "split target was replaced") {
		t.Fatalf("move race error: %v", err)
	}
	after := runTmux(t, binary, socket, "list-panes", "-a", "-F", "#{pane_id}")
	if len(strings.Fields(after)) != len(strings.Fields(before)) {
		t.Fatalf("race created unregistered sibling: before=%q after=%q", before, after)
	}
}
