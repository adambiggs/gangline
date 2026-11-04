package main

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
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

var symlinkedLeader = substrate.Process{PID: 10, GroupID: 10, Command: "2.1.288", Name: "2.1.288"}

// tmux on macOS reports a symlinked harness under its link target's name.
func TestHarnessForegroundIdentifiesSymlinkedLeader(t *testing.T) {
	leader := symlinkedLeader
	for _, tc := range []struct {
		name    string
		fixture foregroundFixture
		want    string
	}{
		{"tmux names the harness", foregroundFixture{command: "claude"}, "claude"},
		{"leader matches resolved executable", foregroundFixture{command: "2.1.288", processes: []substrate.Process{leader}}, "2.1.288"},
		{"leader named otherwise", foregroundFixture{command: "vim", processes: []substrate.Process{leader}}, ""},
		{"member that does not lead", foregroundFixture{command: "2.1.288", processes: []substrate.Process{{PID: 9, GroupID: 9, Command: "sh", Name: "sh"}, {PID: 11, GroupID: 9, Command: "claude", Name: "2.1.288"}}}, ""},
		{"leader named another command", foregroundFixture{command: "2.1.288", processes: []substrate.Process{{PID: 10, GroupID: 10, Command: "codex", Name: "codex"}}}, ""},
		{"process table unavailable", foregroundFixture{command: "2.1.288", processErr: errors.New("not visible")}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := harnessForegroundNamed(context.Background(), tc.fixture, "%1", "claude", map[string]bool{"claude": true, "2.1.288": true})
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
		registry:     registry, identity: tmux.PaneIdentity{Pane: "%1"}, launch: "2.1.288",
	}
	if err := input.SendKeys(context.Background(), "%1", substrate.Keys{Text: "typed"}); err != nil {
		t.Fatal(err)
	}
	if registry.name != "2.1.288" {
		t.Fatalf("registered keys sent under foreground name %q, want %q", registry.name, "2.1.288")
	}
}

func TestLaunchCommandNamesResolveConfiguredPath(t *testing.T) {
	names := launchCommandNames("/tools/claude", func(path string) (string, error) {
		if path != "/tools/claude" {
			t.Fatalf("resolved %q", path)
		}
		return "/tools/versions/2.1.288", nil
	})
	if len(names) != 2 || !names["claude"] || !names["2.1.288"] {
		t.Fatalf("names = %v", names)
	}
	missing := launchCommandNames("claude", func(string) (string, error) { return "", errors.New("missing") })
	if len(missing) != 1 || !missing["claude"] {
		t.Fatalf("missing executable names = %v", missing)
	}
}

func TestHarnessForegroundRefusesUnresolvedOrTruncatedAlias(t *testing.T) {
	for _, name := range []string{"version-unknown", "long-version-na"} {
		f := foregroundFixture{command: name, processes: []substrate.Process{{PID: 10, GroupID: 10, Command: name, Name: name}}}
		_, err := harnessForegroundNamed(context.Background(), f, "%1", "claude", map[string]bool{"claude": true, "long-version-name": true})
		if err == nil {
			t.Fatalf("unmatched kernel name %q accepted", name)
		}
	}
}

// A running registration keeps its old executable name across upgrades and
// serialization. Rebinding the same pane ID must use the new registration.
func TestRegisteredForegroundKeepsLaunchNameUntilRegistrationReplaced(t *testing.T) {
	registry := &sentNameRegistry{}
	run := runtime{cmd: command{paneBackend: registry}}
	a := core.Agent{Pane: "%1", Collar: "claude", Registration: core.PaneRegistration{
		Generation: "old-generation", Session: "$1", TokenHash: "token", LaunchCommand: "2.1.288",
	}}
	data, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &a); err != nil {
		t.Fatal(err)
	}
	fixture := foregroundFixture{command: "2.1.288", processes: []substrate.Process{symlinkedLeader}}
	input := run.registeredInput(a, fixture)
	// Current launch metadata may now point elsewhere. No check of the saved
	// registration may resolve it again or use its new alias.
	current := harness.Collar{Launch: harness.Launch{Command: "/new-install/claude"}}
	if err := sendHarnessKeys(context.Background(), input, "%1", current, substrate.Keys{Text: "typed"}); err != nil {
		t.Fatal(err)
	}
	if registry.name != "2.1.288" {
		t.Fatalf("sent under %q", registry.name)
	}
	a.Registration.Generation = "replacement-generation"
	a.Registration.LaunchCommand = "2.1.289"
	replacement := run.registeredInput(a, input)
	if err := requireHarnessForeground(context.Background(), replacement, "%1", current); err == nil {
		t.Fatal("replacement registration accepted the old process name")
	}
	if err := input.SendKeys(context.Background(), "%2", substrate.Keys{}); err == nil {
		t.Fatal("saved launch name authorized another pane")
	}
}

func TestLegacyRegistrationDoesNotResolveNewAlias(t *testing.T) {
	run := runtime{cmd: command{paneBackend: &sentNameRegistry{}}}
	a := core.Agent{Pane: "%1", Collar: "claude", Registration: core.PaneRegistration{Generation: "old", Session: "$1", TokenHash: "token"}}
	current := harness.Collar{Launch: harness.Launch{Command: "claude"}}
	for _, name := range []string{"claude", "2.1.288", "2.1.289"} {
		input := run.registeredInput(a, foregroundFixture{command: name, processes: []substrate.Process{{PID: 1, GroupID: 1, Command: name, Name: name}}})
		err := requireHarnessForeground(context.Background(), input, "%1", current)
		if (err == nil) != (name == "claude") {
			t.Fatalf("legacy name %q: %v", name, err)
		}
	}
	a.Registration = core.PaneRegistration{}
	if err := requireHarnessForeground(context.Background(), run.registeredInput(a, foregroundFixture{command: "claude"}), "%1", current); err == nil {
		t.Fatal("missing registration accepted")
	}
}
