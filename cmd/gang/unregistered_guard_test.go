package main

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/adambiggs/gangline/substrate"
)

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

func TestUnregisteredPaneRequiresIdentifiedShellTree(t *testing.T) {
	commands := map[string]bool{"codex": true, "claude": true, "custom": true, "2.1.289": true}
	shell := substrate.Process{PID: 1, Command: "zsh", Name: "zsh"}
	for _, tc := range []struct {
		name                   string
		f                      unregisteredFixture
		wantAgent, wantRefusal bool
	}{
		{"shell only", unregisteredFixture{command: "zsh", visible: true, processes: []substrate.Process{shell}}, false, false},
		{"foreground harness", unregisteredFixture{command: "codex"}, true, false},
		{"resolved harness", unregisteredFixture{command: "2.1.289"}, true, false},
		{"custom harness", unregisteredFixture{command: "custom"}, true, false},
		{"background harness", unregisteredFixture{command: "zsh", visible: true, processes: []substrate.Process{shell, {Command: "codex", Name: "codex"}}}, true, false},
		{"old foreground after update", unregisteredFixture{command: "2.1.288"}, false, true},
		{"old background after update", unregisteredFixture{command: "zsh", visible: true, processes: []substrate.Process{shell, {Command: "2.1.288", Name: "2.1.288"}}}, false, true},
		{"unknown foreground", unregisteredFixture{command: "vim"}, false, true},
		{"private namespace", unregisteredFixture{command: "zsh"}, false, true},
		{"empty tree", unregisteredFixture{command: "zsh", visible: true}, false, true},
		{"unreadable tree", unregisteredFixture{command: "zsh", visible: true, err: errors.New("tree unavailable")}, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := unregisteredAgent(context.Background(), tc.f, "%1", commands)
			if got != tc.wantAgent || (err != nil) != tc.wantRefusal {
				t.Fatalf("agent=%v err=%v; want agent=%v refusal=%v", got, err, tc.wantAgent, tc.wantRefusal)
			}
			if tc.wantRefusal && (!strings.Contains(err.Error(), "close that exact pane") || !strings.Contains(err.Error(), "%1")) {
				t.Fatalf("missing recovery action or pane: %v", err)
			}
			if tc.f.err != nil && !errors.Is(err, tc.f.err) {
				t.Fatalf("lost read error: %v", err)
			}
		})
	}
}
