//go:build acceptance

package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/adambiggs/gangline/core"
)

type partialResult struct {
	Agents    []agentJSON      `json:"agents"`
	Total     int              `json:"total"`
	Truncated bool             `json:"truncated"`
	Partial   *bool            `json:"partial"`
	Errors    []map[string]any `json:"errors"`
}

func readPartialRoster(t *testing.T, f *stateFixture, args ...string) partialResult {
	t.Helper()
	f.out.Reset()
	f.errOut.Reset()
	if len(args) == 0 {
		args = []string{"--all"}
	}
	if err := f.cmd.execute(append([]string{"roster", "--json", "--allow-partial"}, args...)); err != nil {
		t.Fatalf("partial roster: %v", err)
	}
	var got partialResult
	if err := json.Unmarshal([]byte(f.out.String()), &got); err != nil {
		t.Fatal(err)
	}
	if got.Partial == nil || got.Errors == nil {
		t.Fatalf("missing partial contract: %s", f.out.String())
	}
	return got
}

func checkPartial(t *testing.T, got partialResult, total int, stages ...string) {
	t.Helper()
	if got.Total != total || *got.Partial != (len(stages) > 0) || len(got.Errors) != len(stages) {
		t.Fatalf("roster=%+v", got)
	}
	for i, e := range got.Errors {
		if len(e) != 4 || e["kind"] != "native" || e["stage"] != stages[i] {
			t.Fatalf("diagnostic=%+v", e)
		}
	}
}

func TestRosterPartialStateRead(t *testing.T) {
	for _, fault := range []string{"malformed", "io"} {
		t.Run(fault, func(t *testing.T) {
			f := newStateFixture(t)
			bad := f.add(t, "private-directory", "untrusted-name", "codex")
			f.add(t, "good", "healthy", "codex")
			p, _ := f.run.team.Agent(bad.ID)
			switch fault {
			case "malformed":
				if err := os.WriteFile(p.State, []byte("private broken record"), 0600); err != nil {
					t.Fatal(err)
				}
			case "io":
				if err := os.Remove(p.State); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(p.State, 0700); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := f.run.team.ListAgents(); err == nil {
				t.Fatal("strict inventory accepted bad state")
			}
			if err := f.cmd.execute([]string{"roster", "--json"}); err == nil {
				t.Fatal("default roster accepted bad state")
			}
			got := readPartialRoster(t, f)
			checkPartial(t, got, 1, "state_read")
			if len(got.Agents) != 1 || got.Agents[0].Name != "healthy" {
				t.Fatalf("healthy row missing: %+v", got.Agents)
			}
			if got.Errors[0]["hitch_id"] != nil || got.Errors[0]["name"] != nil {
				t.Fatalf("unverified identity: %+v", got.Errors)
			}
			for _, s := range []string{"private-directory", "untrusted-name", "private broken record", p.Directory} {
				if strings.Contains(f.out.String(), s) {
					t.Fatalf("exposed %q", s)
				}
			}
			if f.errOut.Len() != 0 {
				t.Fatal(f.errOut.String())
			}
		})
	}
}

func TestRosterPartialCollarLoad(t *testing.T) {
	f := newStateFixture(t)
	bad := f.add(t, "bad", "missing-collar", "codex-missing-fixture")
	f.add(t, "good", "healthy", "codex")
	for _, args := range [][]string{{"roster", "--json"}, {"roster"}} {
		if err := f.cmd.execute(args); err == nil || !strings.Contains(err.Error(), "collar") {
			t.Fatalf("strict collar failure=%v", err)
		}
	}
	got := readPartialRoster(t, f)
	checkPartial(t, got, 1, "collar_load")
	if got.Errors[0]["hitch_id"] != "bad" || got.Errors[0]["name"] != "missing-collar" {
		t.Fatalf("identity=%+v", got.Errors)
	}
	if f.errOut.Len() != 0 {
		t.Fatal(f.errOut.String())
	}
	p, _ := f.run.team.Agent(bad.ID)
	retained, err := p.Read()
	if err != nil || retained.Status != core.Active {
		t.Fatalf("collar failure fabricated lifecycle state: %+v / %v", retained, err)
	}
	l, err := p.TryLock()
	if err != nil {
		t.Fatalf("lock leaked: %v", err)
	}
	_ = l.Close()
	if err := f.cmd.execute([]string{"status", "missing-collar", "--json"}); err == nil {
		t.Fatal("status became permissive")
	}
	f.out.Reset()
	f.errOut.Reset()
	if err := f.cmd.execute([]string{"roster", "--allow-partial"}); err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"healthy", "incomplete", "collar_load"} {
		if !strings.Contains(f.out.String(), s) {
			t.Fatalf("text=%q", f.out.String())
		}
	}
}

func TestRosterPartialEmptyAndAllFailed(t *testing.T) {
	f := newStateFixture(t)
	checkPartial(t, readPartialRoster(t, f), 0)
	f.add(t, "bad", "bad", "missing-fixture")
	checkPartial(t, readPartialRoster(t, f), 0, "collar_load")
}

func TestRosterPartialDoesNotLoadInactiveOrAbsentCollar(t *testing.T) {
	for _, mode := range []string{"inactive", "absent"} {
		t.Run(mode, func(t *testing.T) {
			f := newStateFixture(t)
			a := f.add(t, "bad", "bad", "missing-fixture")
			saveAgent(t, f, a.ID, func(a *core.Agent) {
				if mode == "inactive" {
					a.Status = core.Failed
				} else {
					a.Pane = ""
				}
			})
			checkPartial(t, readPartialRoster(t, f), 1)
		})
	}
}

func TestRosterPartialRetainsFatalBoundaries(t *testing.T) {
	for _, fault := range []string{"name-binding", "outside", "inventory"} {
		t.Run(fault, func(t *testing.T) {
			f := newStateFixture(t)
			f.add(t, "good", "healthy", "codex")
			var path string
			switch fault {
			case "name-binding":
				path = filepath.Join(f.run.team.Names, "healthy")
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			case "outside":
				path = filepath.Join(f.run.team.Directory, "outside", "outside-fixture.json")
				if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
					t.Fatal(err)
				}
			case "inventory":
				if err := os.Rename(f.run.team.Agents, f.run.team.Agents+"-retained"); err != nil {
					t.Fatal(err)
				}
				path = f.run.team.Agents
			}
			if fault == "name-binding" {
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
			} else if err := os.WriteFile(path, []byte("invalid fixture"), 0600); err != nil {
				t.Fatal(err)
			}
			f.out.Reset()
			if err := f.cmd.execute([]string{"roster", "--json", "--allow-partial"}); err == nil {
				t.Fatalf("%s became partial success: %s", fault, f.out.String())
			}
		})
	}
}

func TestRosterPartialPaginationAndModelFields(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "a", "alpha", "codex")
	f.add(t, "b", "beta", "codex")
	f.add(t, "z", "zulu", "missing-fixture")
	saveAgent(t, f, a.ID, func(a *core.Agent) {
		a.ConfiguredModel, a.ConfiguredEffort = "vendor/configured", "high"
		a.Native.Model, a.Native.Effort = "vendor/observed", "low"
	})
	if err := os.WriteFile(filepath.Join(f.run.team.Directory, "watchdog-unavailable"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	got := readPartialRoster(t, f, "--limit", "1")
	checkPartial(t, got, 2, "collar_load")
	if len(got.Agents) != 1 || got.Agents[0].Name != "alpha" || !got.Truncated {
		t.Fatalf("page=%+v", got)
	}
	row := got.Agents[0]
	values := []*string{row.ConfiguredModel, row.ConfiguredEffort, row.ObservedModel, row.ObservedEffort}
	for i, want := range []string{"vendor/configured", "high", "vendor/observed", "low"} {
		if values[i] == nil || *values[i] != want {
			t.Fatalf("model field %d=%v, want %s", i, values[i], want)
		}
	}
	var raw map[string]any
	if err := json.Unmarshal([]byte(f.out.String()), &raw); err != nil {
		t.Fatal(err)
	}
	if raw["watchdog_available"] != false {
		t.Fatal("watchdog availability lost")
	}
	all := readPartialRoster(t, f)
	checkPartial(t, all, 2, "collar_load")
	if all.Truncated || len(all.Agents) != 2 || !reflect.DeepEqual(got.Errors, all.Errors) {
		t.Fatalf("full inventory=%+v", all)
	}
}

func TestRosterPartialDefaultShapeAndLockedFallback(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "a", "locked", "missing-fixture")
	p, _ := f.run.team.Agent(a.ID)
	l, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	if err := f.cmd.execute([]string{"roster", "--json", "--all"}); err != nil {
		t.Fatal(err)
	}
	var strict map[string]any
	if err := json.Unmarshal([]byte(f.out.String()), &strict); err != nil {
		t.Fatal(err)
	}
	if len(strict) != 4 {
		t.Fatalf("default shape changed: %v", strict)
	}
	for _, key := range []string{"agents", "total", "truncated", "watchdog_available"} {
		if _, ok := strict[key]; !ok {
			t.Fatalf("missing %s", key)
		}
	}
	got := readPartialRoster(t, f)
	checkPartial(t, got, 1)
	if len(got.Agents) != 1 || got.Agents[0].HitchID != a.ID {
		t.Fatalf("locked row lost: %+v", got)
	}
}

func TestRosterPartialPreservesMissingAndStaleSkips(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "a", "missing", "codex")
	p, _ := f.run.team.Agent(a.ID)
	if err := os.Remove(p.State); err != nil {
		t.Fatal(err)
	}
	f.add(t, "b", "stale", "codex")
	binding := filepath.Join(f.run.team.Names, "stale")
	if err := os.Remove(binding); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("other", binding); err != nil {
		t.Fatal(err)
	}
	checkPartial(t, readPartialRoster(t, f), 0)
}

func TestRosterPartialCleanupAndAuditFailuresStayFatal(t *testing.T) {
	t.Run("cleanup", func(t *testing.T) {
		f := newStateFixture(t)
		a := f.add(t, "a", "bad", "missing-fixture")
		sentinel := errors.New("cleanup failed")
		calls := 0
		f.run.cmd.detach = func(string, hookNotice) error { calls++; return sentinel }
		f.run.wake = []core.HitchID{"other"}
		_, _, err := f.run.observeRosterPartial([]core.Agent{a}, true)
		if !errors.Is(err, sentinel) || calls != 1 {
			t.Fatalf("cleanup swallowed or repeated: %v, calls=%d", err, calls)
		}
		p, _ := f.run.team.Agent(a.ID)
		l, err := p.TryLock()
		if err != nil {
			t.Fatalf("lock leaked: %v", err)
		}
		if err := l.Close(); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("audit", func(t *testing.T) {
		f := newStateFixture(t)
		a := f.add(t, "a", "bad", "missing-fixture")
		saveAgent(t, f, a.ID, func(a *core.Agent) {
			a.Activity = core.Interrupting
			a.InterruptDeadline = f.cmd.now()
		})
		// Force deadline persistence to reach a failed audit append before collar loading.
		if err := os.Mkdir(f.run.team.Log, 0700); err != nil {
			t.Fatal(err)
		}
		if err := f.cmd.execute([]string{"roster", "--json", "--allow-partial"}); err == nil {
			t.Fatal("audit failure became partial success")
		}
		if f.out.Len() != 0 {
			t.Fatalf("success output on audit error: %s", f.out.String())
		}
	})
}

func TestRosterPartialMixedAndAllUnreadable(t *testing.T) {
	t.Run("mixed", func(t *testing.T) {
		f := newStateFixture(t)
		f.add(t, "a", "native", "codex")
		registerOutsideFixture(t, f)
		got := readPartialRoster(t, f)
		checkPartial(t, got, 2)
		if len(got.Agents) != 2 || got.Agents[0].Kind != "native" || got.Agents[1].Kind != "outside" {
			t.Fatalf("mixed inventory=%+v", got)
		}
	})
	t.Run("all-unreadable", func(t *testing.T) {
		f := newStateFixture(t)
		a := f.add(t, "a", "unreadable", "codex")
		p, _ := f.run.team.Agent(a.ID)
		if err := os.WriteFile(p.State, []byte("invalid"), 0600); err != nil {
			t.Fatal(err)
		}
		f.add(t, "b", "bad-collar", "missing-fixture")
		got := readPartialRoster(t, f)
		checkPartial(t, got, 0, "collar_load", "state_read")
		if got.Agents == nil || len(got.Agents) != 0 || got.Truncated {
			t.Fatalf("all failed=%+v", got)
		}
	})
}

func TestRosterPartialLaterReadAndSharedTmuxFailuresStayFatal(t *testing.T) {
	t.Run("later-read", func(t *testing.T) {
		f := newStateFixture(t)
		a := f.add(t, "a", "native", "codex")
		agents, failures, err := f.run.team.ListAgentsForRoster()
		if err != nil || failures != 0 || len(agents) != 1 {
			t.Fatalf("inventory: %+v/%d/%v", agents, failures, err)
		}
		p, _ := f.run.team.Agent(a.ID)
		if err := os.Remove(p.State); err != nil {
			t.Fatal(err)
		}
		_, _, err = f.run.observeRosterPartial(agents, true)
		if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("later disappearance swallowed: %v", err)
		}
	})
	t.Run("tmux", func(t *testing.T) {
		f := newStateFixture(t)
		f.add(t, "a", "native", "codex")
		script := "#!/bin/sh\n# SPDX-License-Identifier: Apache-2.0\n" + fakeTmuxUTF8 + "case \"$1\" in has-session) exit 0;; *) exit 91;; esac\n"
		if err := os.WriteFile(f.env["GANG_TMUX"], []byte(script), 0700); err != nil {
			t.Fatal(err)
		}
		if err := f.cmd.execute([]string{"roster", "--json", "--allow-partial"}); err == nil {
			t.Fatal("shared tmux failure became partial success")
		}
		if f.out.Len() != 0 {
			t.Fatalf("success output on tmux error: %s", f.out.String())
		}
	})
}
