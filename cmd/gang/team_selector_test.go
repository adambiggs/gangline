package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/store"
)

// teamSelectorCommands lists every command that acts on one team, with
// operands that its own parser accepts.
var teamSelectorCommands = map[string][]string{
	"up":        {"lead"},
	"hitch":     {"worker"},
	"rename":    {"worker", "scout"},
	"send":      {"worker", "body"},
	"queue":     {},
	"interrupt": {"worker"},
	"compact":   {"worker"},
	"context":   {"worker"},
	"log":       {},
	"limits":    {"worker"},
	"snooze":    {"--status"},
	"wait":      {"worker", "--timeout", "0"},
	"curfew":    {},
	"status":    {"worker"},
	"tick":      {},
	"capture":   {"worker"},
	"whoami":    {},
	"roster":    {},
	"attach":    {},
	"drop":      {"worker"},
	"down":      {"--yes"},
}

// addToTeam registers a worker in another team under the fixture's state root.
func addToTeam(t *testing.T, f *stateFixture, team, id, name string) {
	t.Helper()
	p, err := (store.Paths{Root: f.env["GANG_STATE_ROOT"]}).Team(team)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Create(); err != nil {
		t.Fatal(err)
	}
	a := core.Agent{ID: core.HitchID(id), Name: core.AgentName(name), Collar: "codex", Directory: "/work", Pane: "%1", Registration: core.PaneRegistration{Generation: strings.Repeat("a", 64), Session: "$1", TokenHash: tokenHash("fixture-nonce")}, Process: core.ProcessIdentity{PID: 7, Started: "fixture", BootID: "fixture", Namespace: "fixture"}, Status: core.Active, Activity: core.Idle, CreatedAt: f.cmd.now(), ChangedAt: f.cmd.now()}
	l, err := p.CreateAgent(a)
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestTeamFlagSelectsTeamOutsidePane(t *testing.T) {
	f := newStateFixture(t)
	f.add(t, "a", "home-worker", "codex")
	addToTeam(t, f, "other", "b", "other-worker")
	for _, args := range [][]string{
		{"roster"},
		{"roster", "--team", "other"},
		{"roster", "--team=other"},
		{"roster", "-team", "other"},
		{"roster", "--json", "--team", "other"},
		{"roster", "--team", "other", "--json"},
	} {
		f.out.Reset()
		if err := f.cmd.execute(args); err != nil {
			t.Fatalf("%q: %v", args, err)
		}
		want, absent := "home-worker", "other-worker"
		if len(args) > 1 {
			want, absent = absent, want
		}
		if !strings.Contains(f.out.String(), want) || strings.Contains(f.out.String(), absent) {
			t.Errorf("%q printed %q, want %s only", args, f.out.String(), want)
		}
	}
	f.out.Reset()
	if err := f.cmd.execute([]string{"config", "--team", "other"}); err == nil {
		t.Errorf("config accepted --team")
	}
}

func TestTeamFlagKeepsOperandsAndOptionValues(t *testing.T) {
	f := newStateFixture(t)
	f.add(t, "a", "worker", "codex")
	addToTeam(t, f, "other", "b", "worker")
	// A --team after -- is an operand, and a --team that is another option's
	// value stays that option's value.
	if err := f.cmd.execute([]string{"roster", "--", "--team", "other"}); err == nil || !strings.Contains(err.Error(), `unexpected argument "--team"`) {
		t.Errorf("roster -- --team = %v, want the operand refused", err)
	}
	if err := f.cmd.execute([]string{"send", "worker", "--from", "--team", "other"}); err == nil || !strings.Contains(err.Error(), "--from") {
		t.Errorf("send --from --team = %v, want --team read as the --from value", err)
	}
	var usage commandError
	if err := f.cmd.execute([]string{"log", "--agent", "--team"}); errors.As(err, &usage) && usage.status == exitUsage {
		t.Errorf("log --agent --team = %v, want --team read as the agent filter", err)
	}
}

func TestTeamFlagRequiresValue(t *testing.T) {
	f := newStateFixture(t)
	for _, args := range [][]string{{"roster", "--team"}, {"roster", "--team="}, {"roster", "--team", ""}, {"roster", "--team", " "}, {"roster", "---team", "unit"}, {"log", "--team", "unit", "log.jsonl"}} {
		var usage commandError
		if err := f.cmd.execute(args); !errors.As(err, &usage) || usage.status != exitUsage || !strings.Contains(err.Error(), "team") {
			t.Errorf("%q = %v, want a usage error naming the team", args, err)
		}
	}
}

func TestTeamFlagReachesEveryTeamCommand(t *testing.T) {
	for name := range commandUsage {
		if _, listed := teamSelectorCommands[name]; listed != teamCommands[name] {
			t.Errorf("%s accepts --team: %v, listed here: %v", name, teamCommands[name], listed)
		}
	}
	for name, operands := range teamSelectorCommands {
		t.Run(name, func(t *testing.T) {
			f := newStateFixture(t)
			// No team may hold this path segment, so only a command that
			// selects the flagged team reaches this error.
			args := append([]string{name, "--team", ".."}, operands...)
			if err := f.cmd.execute(args); err == nil || !strings.Contains(err.Error(), `invalid state path segment ".."`) {
				t.Fatalf("%q = %v, want the flagged team selected", args, err)
			}
		})
	}
}

func TestTeamFlagInHitchedPaneStaysOnItsTeam(t *testing.T) {
	for name, operands := range teamSelectorCommands {
		t.Run(name, func(t *testing.T) {
			f := newStateFixture(t)
			f.env["GANG_AGENT_ID"] = "a"
			args := append([]string{name, "--team", "other"}, operands...)
			var refused commandError
			if err := f.cmd.execute(args); !errors.As(err, &refused) || refused.status != exitRefused || !strings.Contains(err.Error(), `"unit"`) || !strings.Contains(err.Error(), `"other"`) || !strings.Contains(err.Error(), "GANG_SESSION") {
				t.Fatalf("%q in a hitched pane = %v, want a refusal naming both teams and the route", args, err)
			}
		})
	}
	f := newStateFixture(t)
	f.add(t, "a", "worker", "codex")
	f.env["GANG_AGENT_ID"] = "a"
	if err := f.cmd.execute([]string{"roster", "--team", "unit"}); err != nil || !strings.Contains(f.out.String(), "worker") {
		t.Errorf("roster --team naming the selected team = %v, output %q", err, f.out.String())
	}
}

func TestTeamFlagRefusedByCommandsWithoutTeam(t *testing.T) {
	f := newStateFixture(t)
	for _, name := range []string{"teams", "collars", "models", "roles", "config", "upgrade", "statusline"} {
		for _, selector := range [][]string{{"--team", "other"}, {"--team=other"}} {
			var usage commandError
			if err := f.cmd.execute(append([]string{name}, selector...)); !errors.As(err, &usage) || usage.status != exitUsage {
				t.Errorf("%s %q = %v, want a usage error", name, selector, err)
			}
		}
	}
}

func TestHelpStatesAgentAndTeamSeparately(t *testing.T) {
	for name := range commandUsage {
		var stdout, stderr bytes.Buffer
		if status := run([]string{"help", name}, strings.NewReader(""), &stdout, &stderr); status != exitOK {
			t.Fatalf("help %s: status=%d stderr=%q", name, status, stderr.String())
		}
		help := stdout.String()
		for _, caveat := range []string{"not the team itself", "not a team session", "not the team session"} {
			if strings.Contains(help, caveat) {
				t.Errorf("help %s keeps %q", name, caveat)
			}
		}
		_, team := teamSelectorCommands[name]
		usage := commandUsage[name]
		if got := strings.Count(usage, "[--team TEAM]"); team && got != strings.Count(usage, "gang "+name) || !team && got != 0 {
			t.Errorf("help %s shows [--team TEAM] in %d of its forms", name, got)
		}
		if got := helpHasOptionRow(help, optionSpec{"team", "TEAM", "team to act on"}); got != team {
			t.Errorf("help %s lists --team: %v", name, got)
		}
	}
	if strings.Contains(welcomeHelp, "team session") {
		t.Errorf("welcome help keeps the team-session caveat")
	}
}

func TestTeamFlagReachesWatchdogEnvironment(t *testing.T) {
	f, s := watchdogFixture(t)
	addToTeam(t, f, "other", "b", "other-worker")
	if err := f.cmd.execute([]string{"tick", "--team", "other"}); err != nil {
		t.Fatal(err)
	}
	if s.armed == "" || s.environment["GANG_SESSION"] != "other" {
		t.Fatalf("watchdog for tick --team other: armed %q, environment %v", s.armed, s.environment)
	}
}
