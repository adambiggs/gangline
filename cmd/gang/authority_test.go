package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
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

func downRecords(t *testing.T, f *stateFixture) []downRecord {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(f.env["GANG_STATE_ROOT"], "downs.jsonl"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	var records []downRecord
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		var r downRecord
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			t.Fatalf("down record %q: %v", line, err)
		}
		records = append(records, r)
	}
	return records
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
			if records := downRecords(t, f); len(records) != 0 {
				t.Fatalf("refused down was recorded: %+v", records)
			}
		})
	}
}

func TestDownByLeadOperatorOrAnotherTeamIsRecorded(t *testing.T) {
	for _, test := range []struct{ name, identity, caller string }{
		{"lead", "l", "lead"},
		{"operator", "", ""},
		{"agent of another team", "elsewhere", ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := newStateFixture(t)
			f.addHitched(t, "l", "lead", "lead", "")
			f.addHitched(t, "w", "worker", "worker", "l")
			f.env["GANG_AGENT_ID"] = test.identity
			f.env["TMUX_PANE"] = "%9"
			if err := f.cmd.down([]string{"--yes"}); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(f.run.team.Directory); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("team directory remains or cannot be checked: %v", err)
			}
			records := downRecords(t, f)
			if len(records) != 1 {
				t.Fatalf("down records = %+v", records)
			}
			r := records[0]
			if r.Team != "unit" || r.Agents != 2 || r.Caller != test.caller || r.HitchIdentity != test.identity || r.PID != os.Getpid() || r.ParentPID != os.Getppid() || r.TmuxPane != "%9" || r.Directory != f.env["GANG_STATE_ROOT"] || !r.At.Equal(f.cmd.now()) {
				t.Fatalf("down record = %+v", r)
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

func TestHitchFailureNotifiesHitcher(t *testing.T) {
	for _, test := range []struct {
		name     string
		hitcher  core.HitchID
		hitching core.HitchID
		inactive bool
		notified bool
	}{
		{name: "hitched by an agent", hitcher: "l", notified: true},
		{name: "hitched by the operator", hitcher: ""},
		{name: "failed inside its own hitch command", hitcher: "l", hitching: "o"},
		{name: "hitcher no longer active", hitcher: "l", inactive: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := newStateFixture(t)
			lead := f.addHitched(t, "l", "lead", "lead", "")
			f.addHitched(t, "o", "owner", "", test.hitcher)
			lp, err := f.run.team.Agent(lead.ID)
			if err != nil {
				t.Fatal(err)
			}
			if test.inactive {
				l, err := lp.LockAgent()
				if err != nil {
					t.Fatal(err)
				}
				lead.Status = core.Failed
				if err := l.Save(lead); err != nil {
					t.Fatal(err)
				}
				if err := l.Close(); err != nil {
					t.Fatal(err)
				}
			}
			f.run.hitching = test.hitching
			var woken []string
			f.cmd.detach = func(id string, _ hookNotice) error { woken = append(woken, id); return nil }
			f.run.cmd = f.cmd
			op, err := f.run.team.Agent("o")
			if err != nil {
				t.Fatal(err)
			}
			l, err := op.LockAgent()
			if err != nil {
				t.Fatal(err)
			}
			owner, err := op.Read()
			if err != nil {
				t.Fatal(err)
			}
			if err := f.run.apply(l, &owner, core.Event{Type: "hitch_failed", Reason: "registered pane is absent from tmux"}); err != nil {
				t.Fatal(err)
			}
			if err := f.run.unlock(l); err != nil {
				t.Fatal(err)
			}
			e, err := lp.ReadEnvelope("new", "failed-o")
			if !test.notified {
				if !errors.Is(err, os.ErrNotExist) || len(woken) != 0 {
					t.Fatalf("unexpected notice: %+v %v woken=%v", e, err, woken)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if e.From.Kind != core.SenderGangline || e.Recipient != lead.ID || !strings.Contains(e.Message.Text, "owner, which you hitched, failed: registered pane is absent from tmux") {
				t.Fatalf("notice = %+v", e)
			}
			if len(woken) != 1 || woken[0] != "l" {
				t.Fatalf("woken = %v", woken)
			}
			// A second observation of the same failure adds nothing.
			l, err = op.LockAgent()
			if err != nil {
				t.Fatal(err)
			}
			if err := f.run.apply(l, &owner, core.Event{Type: "hitch_failed", Reason: "again"}); err != nil {
				t.Fatal(err)
			}
			if err := l.Close(); err != nil {
				t.Fatal(err)
			}
			if e2, err := lp.ReadEnvelope("new", "failed-o"); err != nil || e2.Message.Text != e.Message.Text {
				t.Fatalf("repeat notice = %+v %v", e2, err)
			}
		})
	}
}
