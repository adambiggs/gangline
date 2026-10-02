package main

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/substrate"
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

const bootExit = "native process exited with status 1: Do you trust the files in this folder? | No, exit"

// A native CLI that exits at a startup prompt leaves its held pane, and the
// next roster reads the exit as the agent's failure.
func TestRosterReadsExitAtBlockedStartup(t *testing.T) {
	f := newStateFixture(t)
	a := f.setAgent(t, f.add(t, "a", "worker", "codex"), func(a *core.Agent) {
		a.Status, a.Activity, a.Evidence = core.Booting, core.Blocked, "Do you trust the files in this folder?"
	})
	f.input.captureErr = &substrate.ExitedError{Status: "1", Output: "Do you trust the files in this folder?\nNo, exit"}
	if err := f.cmd.roster(nil); err != nil {
		t.Fatal(err)
	}
	got := f.agent(t, a.ID)
	if got.Status != core.Failed || got.Evidence != bootExit {
		t.Fatalf("record = %s %q, want failed with the native exit", got.Status, got.Evidence)
	}
}

// A blocked startup whose pane was closed outside gang fails without its pane.
func TestBlockedStartupWithClosedPaneFails(t *testing.T) {
	f := newStateFixture(t)
	a := f.setAgent(t, f.add(t, "a", "worker", "codex"), func(a *core.Agent) {
		a.Status, a.Activity = core.Booting, core.Blocked
	})
	f.input.captureErr = errors.New("capture pane: can't find pane: %1")
	f.cmd.paneBackend = identityFixture{inputFixture: f.input}
	if err := f.cmd.tick([]string{"--agent", "worker"}); err != nil {
		t.Fatal(err)
	}
	got := f.agent(t, a.ID)
	if got.Status != core.Failed || got.Pane != "" || got.Evidence != "registered pane is absent from tmux" {
		t.Fatalf("record = %s pane %q %q, want failed without its pane", got.Status, got.Pane, got.Evidence)
	}
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

// Startup observed by a tick ends the boot hold, and an exit the hold kept
// fails the agent with its output.
func TestTickReleasesBootHoldAtReadiness(t *testing.T) {
	for _, tc := range []struct {
		name   string
		exit   error
		status core.Status
	}{
		{name: "running", status: core.Active},
		{name: "exited", exit: &substrate.ExitedError{Status: "1", Output: "Do you trust the files in this folder?\nNo, exit"}, status: core.Failed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newStateFixture(t)
			a := f.setAgent(t, f.add(t, "a", "worker", "codex"), func(a *core.Agent) {
				a.Status, a.Activity, a.BootDeadline = core.Booting, core.Unknown, f.cmd.now().Add(bootTimeout)
			})
			f.input.releaseErr = tc.exit
			if err := f.cmd.tick([]string{"--agent", "worker"}); err != nil {
				t.Fatal(err)
			}
			got := f.agent(t, a.ID)
			if f.input.releases != 1 || got.Status != tc.status {
				t.Fatalf("releases = %d, status = %s; want 1, %s", f.input.releases, got.Status, tc.status)
			}
			if tc.exit != nil && got.Evidence != bootExit {
				t.Fatalf("evidence = %q, want the native exit", got.Evidence)
			}
		})
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

// Before its deadline, a blocked startup whose pane still shows the prompt is
// read only for an exit; the prompt is not recorded again.
func TestRosterLeavesLiveBlockedStartupAlone(t *testing.T) {
	f := newStateFixture(t)
	a := f.setAgent(t, f.add(t, "a", "worker", "codex"), func(a *core.Agent) {
		a.Status, a.Activity, a.Evidence = core.Booting, core.Blocked, "Codex folder trust is required"
	})
	f.input.screen = screenWithText("Trust this folder?", "› 1. Trust and continue")
	before := eventsOfType(t, f, "hitch_blocked")
	if err := f.cmd.roster(nil); err != nil {
		t.Fatal(err)
	}
	if got := f.agent(t, a.ID); got.Status != core.Booting || got.Activity != core.Blocked {
		t.Fatalf("record = %s %s, want booting blocked", got.Status, got.Activity)
	}
	if after := eventsOfType(t, f, "hitch_blocked"); after != before {
		t.Fatalf("roster recorded the prompt again: %d hitch_blocked events, had %d", after, before)
	}
}

func eventsOfType(t *testing.T, f *stateFixture, kind string) int {
	t.Helper()
	data, err := os.ReadFile(f.run.team.Log)
	if errors.Is(err, os.ErrNotExist) {
		return 0
	}
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		e, err := core.DecodeEvent([]byte(line))
		if err != nil {
			t.Fatal(err)
		}
		if e.Type == kind {
			count++
		}
	}
	return count
}
