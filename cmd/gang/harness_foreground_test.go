package main

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/adambiggs/gangline/substrate"
	"github.com/adambiggs/gangline/substrate/tmux"
)

type foregroundFixture struct {
	harnessInput
	command    string
	processes  []substrate.Process
	processErr error
}

func (f foregroundFixture) ForegroundCommand(context.Context, substrate.PaneID) (string, error) {
	return f.command, nil
}
func (f foregroundFixture) ForegroundProcesses(context.Context, substrate.PaneID) ([]substrate.Process, error) {
	return f.processes, f.processErr
}

var symlinkedLeader = substrate.Process{PID: 10, GroupID: 10, Command: "/Users/me/.local/bin/claude", Name: "2.1.288"}

// tmux on macOS reports a symlinked harness under its link target's name.
func TestHarnessForegroundIdentifiesSymlinkedLeader(t *testing.T) {
	leader := symlinkedLeader
	for _, tc := range []struct {
		name    string
		fixture foregroundFixture
		want    string
	}{
		{"tmux names the harness", foregroundFixture{command: "claude"}, "claude"},
		{"leader invoked as the harness", foregroundFixture{command: "2.1.288", processes: []substrate.Process{leader}}, "2.1.288"},
		{"leader named otherwise", foregroundFixture{command: "vim", processes: []substrate.Process{leader}}, ""},
		{"member that does not lead", foregroundFixture{command: "2.1.288", processes: []substrate.Process{{PID: 9, GroupID: 9, Command: "/bin/sh", Name: "2.1.288"}, {PID: 11, GroupID: 9, Command: "claude", Name: "2.1.288"}}}, ""},
		{"leader invoked as another command", foregroundFixture{command: "2.1.288", processes: []substrate.Process{{PID: 10, GroupID: 10, Command: "/bin/codex", Name: "2.1.288"}}}, ""},
		{"process table unavailable", foregroundFixture{command: "2.1.288", processErr: errors.New("not visible")}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := harnessForeground(context.Background(), tc.fixture, "%1", "claude")
			if tc.want == "" {
				if err == nil || !strings.Contains(err.Error(), "refuse input") || !strings.Contains(err.Error(), "tmux reports") {
					t.Fatalf("harnessForeground = %q, %v; want refusal", got, err)
				}
				if tc.fixture.processErr != nil && !errors.Is(err, tc.fixture.processErr) {
					t.Fatalf("refusal %v hides %v", err, tc.fixture.processErr)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Fatalf("harnessForeground = %q, %v; want %q", got, err, tc.want)
			}
		})
	}
}

type sentNameRegistry struct {
	paneRegistry
	name string
}

func (r *sentNameRegistry) SendRegisteredKeys(_ context.Context, _ tmux.PaneIdentity, name string, _ substrate.Keys) error {
	r.name = name
	return nil
}

// tmux checks the foreground again as it types, under the name it reports.
func TestRegisteredKeysCarryTheNameTmuxReports(t *testing.T) {
	registry := &sentNameRegistry{}
	input := paneInput{
		harnessInput: foregroundFixture{command: "2.1.288", processes: []substrate.Process{symlinkedLeader}},
		registry:     registry, identity: tmux.PaneIdentity{Pane: "%1"}, launch: "claude",
	}
	if err := input.SendKeys(context.Background(), "%1", substrate.Keys{Text: "typed"}); err != nil {
		t.Fatal(err)
	}
	if registry.name != "2.1.288" {
		t.Fatalf("registered keys sent under foreground name %q, want %q", registry.name, "2.1.288")
	}
}
