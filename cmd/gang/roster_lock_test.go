package main

import (
	"strings"
	"testing"

	"github.com/adambiggs/gangline/core"
)

// rosterRows runs the roster table and returns each agent's row.
func rosterRows(t *testing.T, f *stateFixture) map[string]string {
	t.Helper()
	f.out.Reset()
	if err := f.cmd.roster(nil); err != nil {
		t.Fatal(err)
	}
	rows := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(f.out.String()), "\n") {
		name, _, _ := strings.Cut(line, " ")
		rows[name] = line
	}
	return rows
}

// A roster or status read while another gang operation holds an agent's lock
// shows the record that operation maintains, with its deadlines read against
// now, and saves nothing.
func TestLockedAgentReadsAsItsRecord(t *testing.T) {
	f := newStateFixture(t)
	held := []core.Agent{
		f.add(t, "a", "idle", "codex"),
		f.setAgent(t, f.add(t, "b", "booting", "codex"), func(a *core.Agent) {
			a.Status, a.Activity, a.BootDeadline = core.Booting, core.Unknown, f.cmd.now().Add(bootTimeout)
		}),
		f.setAgent(t, f.add(t, "c", "blocked", "codex"), func(a *core.Agent) {
			a.Status, a.Activity, a.Evidence = core.Booting, core.Blocked, "Codex folder trust is required"
		}),
		f.setAgent(t, f.add(t, "d", "overdue", "codex"), func(a *core.Agent) {
			a.Status, a.Activity, a.BootDeadline = core.Booting, core.Unknown, f.cmd.now()
		}),
		f.setAgent(t, f.add(t, "e", "wedged", "codex"), func(a *core.Agent) {
			a.Activity, a.Evidence = core.Wedged, "interrupt deadline elapsed"
		}),
	}
	for _, a := range held {
		p, err := f.run.team.Agent(a.ID)
		if err != nil {
			t.Fatal(err)
		}
		l, err := p.TryLock()
		if err != nil {
			t.Fatal(err)
		}
		defer l.Close()
	}
	rows := rosterRows(t, f)
	for name, want := range map[string][]string{
		"idle":    {"idle", "active", "idle", "codex"},
		"booting": {"booting", "booting", "unknown", "codex"},
		"blocked": {"blocked", "booting", "blocked", "codex", "Codex", "folder", "trust", "is", "required"},
		"wedged":  {"wedged", "active", "wedged", "codex", "interrupt", "deadline", "elapsed"},
		"overdue": {"overdue", "failed", "unknown", "codex", "boot", "deadline", "elapsed"},
	} {
		if got := strings.Fields(rows[name]); strings.Join(got, " ") != strings.Join(want, " ") {
			t.Errorf("%s row = %q, want %q", name, got, want)
		}
	}
	f.out.Reset()
	if err := f.cmd.execute([]string{"status", "idle", "--json"}); err != nil {
		t.Fatal(err)
	}
	var status agentJSON
	decodeOutput(t, f, &status)
	if status.Status != core.Active || status.Activity != core.Idle || status.Evidence != "" {
		t.Fatalf("status = %+v, want the recorded active idle agent", status)
	}
	if got := f.agent(t, "d"); got.Status != core.Booting {
		t.Fatalf("a read without the lock saved %s over the booting record", got.Status)
	}
}
