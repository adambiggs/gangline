package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
)

type downPromptReader func([]byte) (int, error)

func (read downPromptReader) Read(buffer []byte) (int, error) { return read(buffer) }

func TestDownConfirmation(t *testing.T) {
	for _, test := range []struct {
		name, input string
		accepted    bool
	}{
		{"yes", "yes\n", true},
		{"y", "y\n", true},
		{"Y", "Y\n", true},
		{"YES", "YES\n", true},
		{"spaced", " Yes \n", true},
		{"no", "no\n", false},
		{"n", "n\n", false},
		{"empty", "\n", false},
		{"eof", "", false},
		{"partial", "yes", false},
		{"extra", "yes please\n", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			var prompt bytes.Buffer
			err := confirmDown(strings.NewReader(test.input), &prompt, "unit", 2)
			if (err == nil) != test.accepted {
				t.Fatalf("confirmation error = %v", err)
			}
			if !strings.Contains(prompt.String(), `"unit"`) || !strings.Contains(prompt.String(), "2 agents") || !strings.HasSuffix(prompt.String(), "[y/N] ") {
				t.Fatalf("prompt = %q", prompt.String())
			}
		})
	}
}

func TestDownWithoutTerminalRequiresYesAndKeepsTeam(t *testing.T) {
	f := newStateFixture(t)
	f.add(t, "a", "worker", "codex")
	err := f.cmd.down(nil)
	var commandErr commandError
	if !errors.As(err, &commandErr) || commandErr.status != exitRefused || !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("nonterminal down error = %v", err)
	}
	agents, err := f.run.team.ListAgents()
	if err != nil || len(agents) != 1 {
		t.Fatalf("team changed: agents=%v, error=%v", agents, err)
	}
	if err := f.cmd.down([]string{"unit"}); err == nil {
		t.Fatal("positional session accepted")
	}
}

func TestDownYesFlagsWithoutTerminal(t *testing.T) {
	for _, flag := range []string{"-y", "--yes"} {
		t.Run(flag, func(t *testing.T) {
			f := newStateFixture(t)
			f.add(t, "a", "worker", "codex")
			if err := f.cmd.down([]string{flag}); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(f.run.team.Directory); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("team directory remains or cannot be checked: %v", err)
			}
		})
	}
}

func TestDownRefusesRegistrationWhileConfirming(t *testing.T) {
	f := newStateFixture(t)
	f.add(t, "a", "worker", "codex")
	f.env["GANG_COLLAR"] = "codex"
	fakeCodexOnPath(t)
	team, err := f.run.team.ReadTeam()
	if err != nil {
		t.Fatal(err)
	}
	team.Curfew = f.cmd.now().Add(-time.Second)
	if err := f.run.team.WriteTeam(team); err != nil {
		t.Fatal(err)
	}
	f.cmd.terminalInput = func() bool { return true }
	var concurrentErrors []error
	f.cmd.stdin = downPromptReader(func(buffer []byte) (int, error) {
		if err := f.run.tickAgent("a", hookNotice{}, false); err != nil {
			t.Fatalf("curfew tick during confirmation: %v", err)
		}
		for _, operation := range []func() error{
			func() error { return f.cmd.hitch([]string{"late"}) },
			func() error { return f.cmd.drop([]string{"worker"}) },
		} {
			concurrentErrors = append(concurrentErrors, operation())
		}
		return strings.NewReader("yes\n").Read(buffer)
	})
	if err := f.cmd.down(nil); err != nil {
		t.Fatal(err)
	}
	for _, registrationErr := range concurrentErrors {
		var refused commandError
		if !errors.As(registrationErr, &refused) || refused.status != exitRefused || !strings.Contains(registrationErr.Error(), "is changing") {
			t.Fatalf("registration error = %v", registrationErr)
		}
	}
	if _, err := os.Stat(f.run.team.Directory); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("team directory remains or cannot be checked: %v", err)
	}
	if _, err := f.run.team.ResolveName("late"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("late registration exists: %v", err)
	}
}

// Each agent down cannot remove gets one line naming it, in the form every
// other line takes, and the refusal keeps its status.
func TestDownNamesEachAgentItCannotRemove(t *testing.T) {
	f := newStateFixture(t)
	for i, name := range []string{"first", "second"} {
		f.setAgent(t, f.add(t, string(rune('a'+i)), name, "codex"), func(a *core.Agent) {
			a.Pane, a.Registration = "%9", core.PaneRegistration{}
		})
	}
	err := f.cmd.down([]string{"--yes"})
	lines := errorLines(err)
	if len(lines) != 2 || errorStatus(err) != exitRefused {
		t.Fatalf("down error lines = %q, status %d", lines, errorStatus(err))
	}
	for i, name := range []string{"first", "second"} {
		if !strings.HasPrefix(lines[i], name+": refuse teardown: incomplete pane registration; inspect retained state at ") {
			t.Fatalf("line %d = %q", i, lines[i])
		}
	}
}

// A refusal keeps down's refused status when an earlier agent failed otherwise.
func TestDownRefusalStatusSurvivesAnEarlierFailure(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root removes a directory its mode protects")
	}
	f := newStateFixture(t)
	f.add(t, "a", "first", "codex")
	f.setAgent(t, f.add(t, "b", "second", "codex"), func(a *core.Agent) { a.Registration = core.PaneRegistration{} })
	p, err := f.run.team.Agent("a")
	if err != nil {
		t.Fatal(err)
	}
	// A directory it cannot empty fails the first agent's drop at its end.
	stuck := filepath.Join(p.Directory, "stuck")
	if err := os.MkdirAll(stuck, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stuck, "file"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(stuck, 0500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(stuck, 0700) })
	err = f.cmd.down([]string{"--yes"})
	lines := errorLines(err)
	if len(lines) != 2 || !strings.HasPrefix(lines[0], "first: ") || !strings.HasPrefix(lines[1], "second: refuse teardown: ") || errorStatus(err) != exitRefused {
		t.Fatalf("down error lines = %q, status %d", lines, errorStatus(err))
	}
}
