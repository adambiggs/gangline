package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/adambiggs/gangline/core"
)

// setAgent rewrites a fixture record under its lock.
func (f *stateFixture) setAgent(t *testing.T, a core.Agent, change func(*core.Agent)) core.Agent {
	t.Helper()
	p, err := f.run.team.Agent(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	l, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	change(&a)
	if err := l.Save(a); err != nil {
		t.Fatal(err)
	}
	return a
}

func (f *stateFixture) agent(t *testing.T, id core.HitchID) core.Agent {
	t.Helper()
	p, err := f.run.team.Agent(id)
	if err != nil {
		t.Fatal(err)
	}
	a, err := p.Read()
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func TestRosterShowsWhyAnAgentIsUnhealthy(t *testing.T) {
	f := newStateFixture(t)
	f.setAgent(t, f.add(t, "a", "failed", "codex"), func(a *core.Agent) {
		a.Status, a.Activity, a.Evidence = core.Failed, core.Unknown, "native process exited with status 1: error:\nunknown model"
	})
	f.setAgent(t, f.add(t, "b", "turnfail", "codex"), func(a *core.Agent) {
		a.Native.TurnFailure = "model_not_found"
	})
	f.add(t, "c", "healthy", "codex")
	if err := f.cmd.roster(nil); err != nil {
		t.Fatal(err)
	}
	rows := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(f.out.String()), "\n") {
		name, _, _ := strings.Cut(line, " ")
		rows[name] = line
	}
	if row := rows["failed"]; !strings.HasSuffix(row, "native process exited with status 1: error: unknown model") {
		t.Fatalf("failed row lacks its reason on one line: %q", row)
	}
	if row := rows["turnfail"]; !strings.Contains(row, " unknown ") || !strings.HasSuffix(row, "native turn failed: model_not_found") {
		t.Fatalf("turn-failed row lacks its reason: %q", row)
	}
	if row := rows["healthy"]; !strings.HasSuffix(row, "codex") {
		t.Fatalf("healthy row carries a reason: %q", row)
	}
}

// A tick or roster that finds an agent's pane closed outside gang fails the
// agent and forgets the pane, so no later observation addresses it.
func TestClosedPaneIsForgotten(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status core.Status
		roster bool
	}{
		{name: "tick active", status: core.Active},
		{name: "tick booting", status: core.Booting},
		{name: "tick failed", status: core.Failed},
		{name: "roster active", status: core.Active, roster: true},
		{name: "roster failed", status: core.Failed, roster: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newStateFixture(t)
			a := f.setAgent(t, f.add(t, "a", "worker", "codex"), func(a *core.Agent) {
				// The fixture's tmux lists only %1.
				a.Pane, a.Status = "%2", tc.status
				if tc.status == core.Booting {
					a.BootDeadline = f.cmd.now().Add(bootTimeout)
				}
				if tc.status == core.Failed {
					a.Evidence = "boot deadline elapsed"
				}
			})
			f.input.captureErr = errors.New("capture pane: can't find pane: %2")
			f.cmd.paneBackend = identityFixture{inputFixture: f.input}
			var err error
			if tc.roster {
				err = f.cmd.roster(nil)
			} else {
				err = f.cmd.tick([]string{"--agent", "worker"})
			}
			if err != nil {
				t.Fatal(err)
			}
			got := f.agent(t, a.ID)
			want := "registered pane is absent from tmux"
			if tc.status == core.Failed {
				want = "boot deadline elapsed"
			}
			if got.Status != core.Failed || got.Pane != "" || got.Evidence != want {
				t.Fatalf("record = %s pane %q %q, want failed without its pane, %q", got.Status, got.Pane, got.Evidence, want)
			}
		})
	}
}
