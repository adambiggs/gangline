package main

import (
	"errors"
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
