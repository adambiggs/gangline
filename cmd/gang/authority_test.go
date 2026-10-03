package main

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/adambiggs/gangline/core"
)

func (f *stateFixture) addHitched(t *testing.T, id, name, role string, hitcher core.HitchID) core.Agent {
	t.Helper()
	a := f.add(t, id, name, "codex")
	p, err := f.run.team.Agent(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	l, err := p.LockAgent()
	if err != nil {
		t.Fatal(err)
	}
	a.Role, a.HitchedBy = role, hitcher
	if err := l.Save(a); err != nil {
		t.Fatal(err)
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	return a
}

func requireRefused(t *testing.T, err error, text string) {
	t.Helper()
	var refused commandError
	if !errors.As(err, &refused) || refused.status != exitRefused || !strings.Contains(err.Error(), text) {
		t.Fatalf("error = %v, want refusal containing %q", err, text)
	}
}

// A shell an owner starts inherits its pane's team and state root, so a
// script it runs that downs "its" team ends the lead and every teammate.
func TestDownRefusesTeammateThatIsNotLead(t *testing.T) {
	for _, test := range []struct{ name, role, nonce string }{
		{"owner", "worker", "fixture-nonce"},
		{"lead without its pane token", "lead", "copied-elsewhere"},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := newStateFixture(t)
			f.addHitched(t, "l", "lead", "lead", "")
			f.addHitched(t, "w", "worker", test.role, "l")
			f.env["GANG_AGENT_ID"] = "w"
			f.env["GANG_AGENT_NONCE"] = test.nonce
			requireRefused(t, f.cmd.down([]string{"--yes"}), "protects every agent in the team")
			if agents, err := f.run.team.ListAgents(); err != nil || len(agents) != 2 {
				t.Fatalf("team changed: agents=%v, error=%v", agents, err)
			}
		})
	}
}

func TestDownByLeadOperatorOrAnotherTeam(t *testing.T) {
	for _, test := range []struct{ name, identity string }{
		{"lead", "l"},
		{"operator", ""},
		{"agent of another team", "elsewhere"},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := newStateFixture(t)
			f.addHitched(t, "l", "lead", "lead", "")
			f.addHitched(t, "w", "worker", "worker", "l")
			f.env["GANG_AGENT_ID"] = test.identity
			if err := f.cmd.down([]string{"--yes"}); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(f.run.team.Directory); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("team directory remains or cannot be checked: %v", err)
			}
		})
	}
}

func TestDropRefusesAgentsTheCallerDidNotHitch(t *testing.T) {
	for _, test := range []struct {
		name, caller, target, refusal string
	}{
		{"teammate's agent", "c", "o", "was hitched by lead, the lead, so only the lead may drop it"},
		{"agent with no hitcher", "o", "x", "records no hitcher, so only the lead may drop it"},
		{"itself", "c", "c", "was hitched by owner"},
		{"own agent without its pane token", "o", "c", "was hitched by owner"},
		{"own agent", "o", "c", ""},
		{"lead drops an agent with no hitcher", "l", "x", ""},
		{"operator", "", "o", ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := newStateFixture(t)
			f.addHitched(t, "l", "lead", "lead", "")
			f.addHitched(t, "o", "owner", "", "l")
			f.addHitched(t, "c", "child", "", "o")
			f.addHitched(t, "x", "legacy", "", "")
			f.env["GANG_AGENT_ID"] = test.caller
			if test.name == "own agent without its pane token" {
				f.env["GANG_AGENT_NONCE"] = "copied-elsewhere"
			}
			names := map[string]string{"o": "owner", "c": "child", "x": "legacy"}
			err := f.cmd.drop([]string{names[test.target]})
			p, pathErr := f.run.team.Agent(core.HitchID(test.target))
			if pathErr != nil {
				t.Fatal(pathErr)
			}
			_, readErr := p.Read()
			if test.refusal != "" {
				requireRefused(t, err, test.refusal)
				if readErr != nil {
					t.Fatalf("refused drop removed the agent: %v", readErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !errors.Is(readErr, os.ErrNotExist) {
				t.Fatalf("dropped agent remains: %v", readErr)
			}
		})
	}
}

func TestCurfewChangeRefusesTeammateThatIsNotLead(t *testing.T) {
	f := newStateFixture(t)
	f.addHitched(t, "l", "lead", "lead", "")
	f.addHitched(t, "w", "worker", "", "l")
	f.env["GANG_AGENT_ID"] = "w"
	for _, args := range [][]string{{"curfew", "2h"}, {"curfew", "clear"}} {
		requireRefused(t, f.cmd.execute(args), "curfew refused")
	}
	if err := f.cmd.execute([]string{"curfew"}); err != nil {
		t.Fatalf("reading the curfew: %v", err)
	}
	team, err := f.run.team.ReadTeam()
	if err != nil || !team.Curfew.IsZero() {
		t.Fatalf("curfew = %v, %v", team.Curfew, err)
	}
	f.env["GANG_AGENT_ID"] = "l"
	if err := f.cmd.execute([]string{"curfew", "2h"}); err != nil {
		t.Fatal(err)
	}
}
