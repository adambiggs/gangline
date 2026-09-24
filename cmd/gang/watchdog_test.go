package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
)

type fakeWatchdog struct {
	armed       string
	arms, stops int
	environment map[string]string
	failure     error
	now         func() time.Time
	due         time.Time
}

func (s *fakeWatchdog) Arm(unit, executable string, environment map[string]string) error {
	if s.failure != nil {
		return s.failure
	}
	if s.armed != "" {
		return errors.New("timer already armed")
	}
	if !filepath.IsAbs(executable) {
		return errors.New("executable is not absolute")
	}
	s.armed, s.environment = unit, environment
	s.arms++
	if s.now != nil {
		s.due = s.now().Add(watchdogTimeout)
	}
	return nil
}
func (s *fakeWatchdog) Disarm(unit string) error {
	if s.failure != nil {
		return s.failure
	}
	if s.armed != "" && s.armed != unit {
		return errors.New("wrong timer")
	}
	s.armed = ""
	s.stops++
	return nil
}
func watchdogFixture(t *testing.T) (*stateFixture, *fakeWatchdog) {
	t.Helper()
	f := newStateFixture(t)
	s := &fakeWatchdog{now: f.cmd.now}
	f.cmd.newScheduler = func() watchdogScheduler { return s }
	f.run.cmd = f.cmd
	f.add(t, "a", "worker", "codex")
	return f, s
}
func TestWatchdogArmsReplacesAndRearms(t *testing.T) {
	f, s := watchdogFixture(t)
	now := f.cmd.now()
	f.cmd.clock = func() time.Time { return now }
	s.now = f.cmd.now
	if err := f.cmd.tick(nil); err != nil {
		t.Fatal(err)
	}
	first, due := s.armed, s.due
	if first == "" || !due.Equal(now.Add(watchdogTimeout)) {
		t.Fatalf("timer: %+v", s)
	}
	// Advance the fake clock directly. No wall-clock deadline is measured.
	now = now.Add(20 * time.Second)
	if err := f.cmd.tick(nil); err != nil {
		t.Fatal(err)
	}
	if s.armed == first || s.arms != 2 || s.stops != 1 || !s.due.Equal(due.Add(20*time.Second)) {
		t.Fatalf("replacement: %+v", s)
	}
	if err := f.cmd.tick([]string{"--source", "watchdog", "--watchdog", first}); err != nil {
		t.Fatal(err)
	}
	if s.arms != 2 {
		t.Fatal("stale generation rearmed")
	}
	current := s.armed
	now = s.due
	if err := f.cmd.tick([]string{"--source", "watchdog", "--watchdog", current}); err != nil {
		t.Fatal(err)
	}
	if s.arms != 3 || s.stops != 2 || s.armed == current {
		t.Fatalf("self rearm: %+v", s)
	}
	if s.environment["GANG_SESSION"] != "unit" || s.environment["GANG_STATE_ROOT"] != f.env["GANG_STATE_ROOT"] {
		t.Fatalf("wrong target: %v", s.environment)
	}
}
func TestWatchdogSources(t *testing.T) {
	f, s := watchdogFixture(t)
	if err := f.cmd.tick(nil); err != nil {
		t.Fatal(err)
	}
	f.env["GANGLINE_BOUNDARY"] = `{"kind":"turn-started"}`
	if err := f.cmd.tick([]string{"--agent", "a"}); err != nil {
		t.Fatal(err)
	}
	if err := f.cmd.tick([]string{"--source", "watchdog", "--watchdog", s.armed}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(f.run.team.Log)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		var e core.Event
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			t.Fatal(err)
		}
		if e.Type == "tick" {
			got = append(got, e.Source)
		}
	}
	if strings.Join(got, ",") != "command,hook,watchdog" {
		t.Fatalf("sources: %v", got)
	}
}
func TestWatchdogDoesNotWaitOnAgentLock(t *testing.T) {
	for _, curfew := range []bool{false, true} {
		t.Run(map[bool]string{false: "ordinary", true: "curfew"}[curfew], func(t *testing.T) {
			f, s := watchdogFixture(t)
			if curfew {
				if err := f.run.team.WriteTeam(core.Team{Curfew: f.cmd.now()}); err != nil {
					t.Fatal(err)
				}
			}
			p, _ := f.run.team.Agent("a")
			lock, err := p.TryLock()
			if err != nil {
				t.Fatal(err)
			}
			defer lock.Close()
			before, err := os.ReadFile(p.State)
			if err != nil {
				t.Fatal(err)
			}
			if err := f.cmd.tick(nil); err != nil {
				t.Fatal(err)
			}
			if err := f.cmd.tick([]string{"--source", "watchdog", "--watchdog", s.armed}); err != nil {
				t.Fatal(err)
			}
			after, err := os.ReadFile(p.State)
			if err != nil {
				t.Fatal(err)
			}
			if string(before) != string(after) || s.arms != 2 {
				t.Fatal("locked agent changed or watchdog did not rearm")
			}
		})
	}
}
func TestWatchdogLastDropAndDownDisarm(t *testing.T) {
	for _, action := range []string{"drop", "down", "missing-state"} {
		t.Run(action, func(t *testing.T) {
			f, s := watchdogFixture(t)
			f.add(t, "b", "other", "codex")
			if err := f.cmd.tick(nil); err != nil {
				t.Fatal(err)
			}
			if err := f.cmd.drop([]string{"other"}); err != nil {
				t.Fatal(err)
			}
			if s.armed == "" {
				t.Fatal("non-last drop disarmed")
			}
			switch action {
			case "drop":
				if err := f.cmd.drop([]string{"worker"}); err != nil {
					t.Fatal(err)
				}
			case "down":
				if err := f.cmd.down([]string{"unit"}); err != nil {
					t.Fatal(err)
				}
			case "missing-state":
				p, _ := f.run.team.Agent("a")
				if err := os.RemoveAll(p.Directory); err != nil {
					t.Fatal(err)
				}
				if err := f.cmd.drop([]string{"worker"}); err != nil {
					t.Fatal(err)
				}
			}
			if s.armed != "" || s.stops != 1 {
				t.Fatalf("not disarmed: %+v", s)
			}
			if err := f.cmd.tick([]string{"--source", "watchdog", "--watchdog", "stale"}); err != nil {
				t.Fatal(err)
			}
			if s.arms != 1 {
				t.Fatal("late timer resurrected team")
			}
		})
	}
}
func TestWatchdogSchedulerLockNeverWaits(t *testing.T) {
	f, s := watchdogFixture(t)
	if err := f.cmd.tick(nil); err != nil {
		t.Fatal(err)
	}
	lock, err := os.OpenFile(filepath.Join(f.run.team.Directory, "watchdog.lock"), os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatal(err)
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	if err := f.cmd.tick([]string{"--source", "watchdog", "--watchdog", s.armed}); err != nil {
		t.Fatal(err)
	}
	if err := f.cmd.tick(nil); err != nil {
		t.Fatal(err)
	}
	if s.arms != 1 {
		t.Fatal("replacement ignored lock")
	}
}
func TestWatchdogUnavailableLogsOnceAndStillTicks(t *testing.T) {
	f, _ := watchdogFixture(t)
	f.cmd.newScheduler = func() watchdogScheduler { return nil }
	for range 2 {
		if err := f.cmd.tick(nil); err != nil {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile(f.run.team.Log)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(data), `"type":"watchdog_unavailable"`) != 1 || strings.Count(string(data), `"type":"tick"`) != 2 {
		t.Fatalf("events: %s", data)
	}
	if err := f.cmd.drop([]string{"worker"}); err != nil {
		t.Fatal(err)
	}
}
func TestWatchdogSchedulerErrorsAreLoud(t *testing.T) {
	f, s := watchdogFixture(t)
	s.failure = errors.New("scheduler failed")
	if err := f.cmd.tick(nil); !errors.Is(err, s.failure) {
		t.Fatalf("lost arm error: %v", err)
	}
	s.failure = nil
	if err := f.cmd.tick(nil); err != nil {
		t.Fatal(err)
	}
	s.failure = errors.New("stop failed")
	if err := f.cmd.drop([]string{"worker"}); !errors.Is(err, s.failure) {
		t.Fatalf("lost cleanup error: %v", err)
	}
}

func TestWatchdogScopedTicksDoNotPostponeIdlePeers(t *testing.T) {
	f, s := watchdogFixture(t)
	if err := f.cmd.tick(nil); err != nil {
		t.Fatal(err)
	}
	unit, due := s.armed, s.due
	for range 3 {
		if err := f.cmd.tick([]string{"--agent", "a"}); err != nil {
			t.Fatal(err)
		}
	}
	if s.armed != unit || s.arms != 1 || !s.due.Equal(due) {
		t.Fatal("scoped tick postponed team watchdog")
	}
}

func TestWatchdogDownDisarmsOnceAfterParallelDrops(t *testing.T) {
	f, s := watchdogFixture(t)
	f.add(t, "b", "second", "codex")
	if err := f.cmd.tick(nil); err != nil {
		t.Fatal(err)
	}
	if err := f.cmd.down([]string{"unit"}); err != nil {
		t.Fatal(err)
	}
	if s.stops != 1 || s.armed != "" {
		t.Fatalf("cleanup: %+v", s)
	}
	if _, err := os.Stat(f.run.team.Directory); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("team remains: %v", err)
	}
}
func TestWatchdogFailureStillTicksAgent(t *testing.T) {
	f, s := watchdogFixture(t)
	s.failure = errors.New("scheduler failed")
	p, _ := f.run.team.Agent("a")
	lock, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	a, err := p.Read()
	if err != nil {
		t.Fatal(err)
	}
	a.Activity = core.Unknown
	if err := lock.Save(a); err != nil {
		t.Fatal(err)
	}
	if err := lock.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.cmd.tick(nil); !errors.Is(err, s.failure) {
		t.Fatalf("scheduler error lost: %v", err)
	}
	a, err = p.Read()
	if err != nil {
		t.Fatal(err)
	}
	if a.Activity != core.Idle {
		t.Fatalf("tick skipped work: %+v", a)
	}
}
