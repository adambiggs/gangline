//go:build acceptance

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/store"
	"github.com/adambiggs/gangline/substrate/tmux"
)

type downPromptReader func([]byte) (int, error)

func (read downPromptReader) Read(buffer []byte) (int, error) { return read(buffer) }

type downOrderRegistry struct {
	paneRegistry
	remove func(tmux.PaneIdentity) error
}

func (b downOrderRegistry) RemoveRegisteredNativePane(_ context.Context, pane tmux.PaneIdentity, _ tmux.Identity) error {
	return b.remove(pane)
}

func TestDownFinishesPeersBeforeEndingTheCallerDespiteFailures(t *testing.T) {
	for _, key := range []string{"GANG_AGENT_ID", "TMUX_PANE"} {
		t.Run(key, func(t *testing.T) {
			f := newStateFixture(t)
			caller := f.addHitched(t, "c", "caller", "lead", "")
			f.setAgent(t, caller, func(a *core.Agent) { a.Pane = "%3" })
			for i, name := range []string{"first", "second"} {
				f.setAgent(t, f.add(t, name, name, "codex"), func(a *core.Agent) { a.Pane = fmt.Sprintf("%%%d", i+1) })
			}
			if key == "GANG_AGENT_ID" {
				f.env[key] = string(caller.ID)
			} else {
				f.env[key] = "%3"
			}
			peersStarted := make(chan struct{})
			var started, finished atomic.Int32
			var called atomic.Bool
			peerFailure := refuseError("peer teardown failed")
			callerFailure := errors.New("caller teardown failed")
			f.cmd.paneBackend = downOrderRegistry{paneRegistry: f.input, remove: func(pane tmux.PaneIdentity) error {
				if pane.Pane == "%3" {
					called.Store(true)
					if got := finished.Load(); got != 2 {
						return fmt.Errorf("caller ended with %d peers finished", got)
					}
					return callerFailure
				}
				// Peers must still run concurrently, so each can reach its
				// completion barrier while the other operation is active.
				if started.Add(1) == 2 {
					close(peersStarted)
				}
				<-peersStarted
				finished.Add(1)
				return peerFailure
			}}
			err := f.cmd.down([]string{"--yes"})
			if !called.Load() || finished.Load() != 2 || errorStatus(err) != exitRefused {
				t.Fatalf("drop attempts: caller=%v peers=%d error=%v", called.Load(), finished.Load(), err)
			}
			if !strings.Contains(err.Error(), "caller: "+callerFailure.Error()) {
				t.Fatalf("caller did not finish after peers: %v", err)
			}
			for _, name := range []string{"first", "second"} {
				if !strings.Contains(err.Error(), name+": "+peerFailure.Error()) {
					t.Fatalf("missing %s failure: %v", name, err)
				}
			}
		})
	}
}

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

func TestDownRetainsAuditAndSeparatesReusedTeam(t *testing.T) {
	f := newStateFixture(t)
	var paths []string
	for _, id := range []string{"first", "second"} {
		if err := f.run.team.Create(); err != nil {
			t.Fatal(err)
		}
		f.add(t, id, "worker", "codex")
		if err := f.run.team.Append(core.Event{Type: "observation", At: f.cmd.now(), HitchID: core.HitchID(id), Reason: "retained proof"}); err != nil {
			t.Fatal(err)
		}
		before, err := os.ReadFile(f.run.team.Log)
		if err != nil {
			t.Fatal(err)
		}
		if err := f.cmd.down([]string{"--yes"}); err != nil {
			t.Fatal(err)
		}
		root := filepath.Dir(filepath.Dir(f.run.team.Directory))
		all, err := filepath.Glob(filepath.Join(root, "logs", filepath.Base(f.run.team.Directory), "*.jsonl"))
		if err != nil || len(all) != len(paths)+1 {
			t.Fatalf("archives: %v, %v", all, err)
		}
		var current string
		for _, path := range all {
			if len(paths) == 0 || path != paths[0] {
				current = path
			}
		}
		retained, err := os.ReadFile(current)
		if err != nil || !bytes.HasPrefix(retained, before) || !bytes.Contains(retained, []byte(`"type":"drop_finished"`)) {
			t.Fatalf("audit missing original or final teardown: %q, %v", retained, err)
		}
		if id == "second" && bytes.Contains(retained, []byte(`"hitch_id":"first"`)) {
			t.Fatal("reused team mixed incarnations")
		}
		data, err := os.ReadFile(filepath.Join(root, "downs.jsonl"))
		if err != nil {
			t.Fatal(err)
		}
		lines := bytes.Split(bytes.TrimSpace(data), []byte("\n"))
		var record downRecord
		if err := json.Unmarshal(lines[len(lines)-1], &record); err != nil || record.RetainedLog != current {
			t.Fatalf("retained path absent from shutdown audit: %+v, %v", record, err)
		}
		f.out.Reset()
		if err := f.cmd.log([]string{"--all", current}); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(f.out.String(), `"type":"drop_finished"`) || !strings.Contains(f.out.String(), "retained proof") {
			t.Fatalf("saved-log reader lost events: %s", f.out.String())
		}
		paths = append(paths, current)
	}
}

func TestDownRetentionFailureLeavesAgentAndAudit(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "codex")
	if err := f.run.team.Append(core.Event{Type: "observation", At: f.cmd.now(), HitchID: a.ID}); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(f.run.team.Log)
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Dir(filepath.Dir(f.run.team.Directory))
	if err := os.WriteFile(filepath.Join(root, "logs"), []byte("not a directory"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := f.cmd.down([]string{"--yes"}); err == nil || !strings.Contains(err.Error(), "retain team log") {
		t.Fatalf("down error: %v", err)
	}
	if got, err := os.ReadFile(f.run.team.Log); err != nil || !bytes.Equal(got, before) {
		t.Fatalf("audit changed: %q, %v", got, err)
	}
	if _, err := f.run.team.ResolveName("worker"); err != nil {
		t.Fatal(err)
	}
}

func TestDropPreservesHoldDiagnosticOrRefusesBeforeStopping(t *testing.T) {
	for _, blocked := range []bool{false, true} {
		t.Run(fmt.Sprint(blocked), func(t *testing.T) {
			f := newStateFixture(t)
			a := f.add(t, "a", "worker", "codex")
			p, err := f.run.team.Agent(a.ID)
			if err != nil {
				t.Fatal(err)
			}
			body := []byte("hold could not set pane option\nexit status 1\n")
			source := filepath.Join(p.Directory, "hold")
			if err := os.WriteFile(source, body, 0600); err != nil {
				t.Fatal(err)
			}
			root := filepath.Dir(filepath.Dir(f.run.team.Directory))
			if blocked {
				if err := os.WriteFile(filepath.Join(root, "logs"), []byte("blocked"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			err = f.cmd.drop([]string{"worker"})
			if blocked {
				if err == nil || f.input.releases != 0 {
					t.Fatalf("drop did not stop before teardown: %v, releases %d", err, f.input.releases)
				}
				if got, err := os.ReadFile(source); err != nil || !bytes.Equal(got, body) {
					t.Fatalf("original diagnostic changed: %q, %v", got, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			paths, err := filepath.Glob(filepath.Join(root, "logs", filepath.Base(f.run.team.Directory), "*-hold-a-*.log"))
			if err != nil || len(paths) != 1 {
				t.Fatalf("retained diagnostics: %v, %v", paths, err)
			}
			if got, err := os.ReadFile(paths[0]); err != nil || !bytes.Equal(got, body) {
				t.Fatalf("retained diagnostic changed: %q, %v", got, err)
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
		// Keep the following operations focused on the confirmation's team lock.
		team.Curfew = time.Time{}
		if err := f.run.team.WriteTeam(team); err != nil {
			t.Fatal(err)
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

func TestDownFailureRetainsItsDiagnosticInTheAudit(t *testing.T) {
	f := newStateFixture(t)
	for _, name := range []string{"first", "second"} {
		f.setAgent(t, f.add(t, name, name, "codex"), func(a *core.Agent) {
			a.Registration = core.PaneRegistration{}
		})
	}
	err := f.cmd.down([]string{"--yes"})
	if err == nil || errorStatus(err) != exitRefused {
		t.Fatalf("down error = %v", err)
	}
	log, readErr := os.ReadFile(f.run.team.Log)
	if readErr != nil {
		t.Fatal(readErr)
	}
	var failures []core.Event
	if readErr := store.ReadLog(bytes.NewReader(log), func(event core.Event) error {
		if event.Type == "down_failed" {
			failures = append(failures, event)
		}
		return nil
	}); readErr != nil {
		t.Fatal(readErr)
	}
	if len(failures) != 1 || failures[0].Reason != err.Error() || !failures[0].At.Equal(f.cmd.now()) {
		t.Fatalf("down failure audit = %+v; want diagnostic %q", failures, err)
	}
	for _, name := range []string{"first", "second"} {
		if !strings.Contains(failures[0].Reason, name+": refuse teardown:") {
			t.Fatalf("down failure does not name %s: %+v", name, failures)
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
