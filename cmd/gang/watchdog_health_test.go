package main

import (
	"errors"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"syscall"
	"testing"

	"github.com/adambiggs/gangline/core"
)

func TestSnoozeWarnsWhenNoWatchdogCanDeliverIt(t *testing.T) {
	f := newStateFixture(t)
	caller := f.add(t, "caller-id", "worker", "codex")
	f.env["GANG_AGENT_ID"] = string(caller.ID)
	f.env["TMUX_PANE"] = caller.Pane
	if err := f.cmd.snooze([]string{"--at", "2h"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(f.errOut.String(), "warning: watchdog unavailable") {
		t.Fatalf("wake accepted without a timer and without a warning: %q", f.errOut)
	}
	if usageSnapshot(t, f.run).Snoozes[string(caller.ID)].ID == "" {
		t.Fatal("warning replaced the wake instead of accompanying it")
	}
}

func TestSnoozeArmsAMissingWatchdog(t *testing.T) {
	f, s := watchdogFixture(t)
	f.env["GANG_AGENT_ID"] = "a"
	f.env["TMUX_PANE"] = "%1"
	if err := f.cmd.snooze([]string{"--at", "2h"}); err != nil {
		t.Fatal(err)
	}
	if s.armed == "" || f.errOut.Len() != 0 {
		t.Fatalf("armed=%q stderr=%q", s.armed, f.errOut)
	}
}

func TestWatchdogUnavailableMarkerClearsWhenTimerArms(t *testing.T) {
	f, s := watchdogFixture(t)
	f.cmd.newScheduler = func() watchdogScheduler { return unavailableWatchdog{s} }
	if err := f.cmd.tick(nil); err != nil {
		t.Fatal(err)
	}
	if err := f.cmd.roster(nil); err != nil || !strings.Contains(f.out.String(), "[watchdog-unavailable]") {
		t.Fatalf("unavailable scheduler not shown: %v %q", err, f.out)
	}
	f.cmd.newScheduler = func() watchdogScheduler { return s }
	if err := f.cmd.tick(nil); err != nil || s.armed == "" {
		t.Fatalf("recovered tick: %v armed=%q", err, s.armed)
	}
	f.out.Reset()
	if err := f.cmd.roster(nil); err != nil || strings.Contains(f.out.String(), "[watchdog-unavailable]") {
		t.Fatalf("marker survived an armed timer: %v %q", err, f.out)
	}
	log, err := os.ReadFile(f.run.team.Log)
	if err != nil || !strings.Contains(string(log), `"type":"watchdog_available"`) {
		t.Fatalf("recovery not recorded: %v %s", err, log)
	}
}

func ownNamespace(t *testing.T) string {
	t.Helper()
	if goruntime.GOOS == "darwin" {
		return "darwin"
	}
	namespace, err := os.Readlink("/proc/self/ns/pid")
	if err != nil {
		t.Fatal(err)
	}
	return namespace
}

// sandboxedVerifiedAgent registers an agent whose own command has run once
// without process visibility, leaving its process-unavailable marker.
func sandboxedVerifiedAgent(t *testing.T, namespace string) (*stateFixture, core.Agent) {
	t.Helper()
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "codex")
	a.Registration = core.PaneRegistration{Generation: "generation", Session: "$1", TokenHash: tokenHash("secret")}
	if namespace != "" {
		a.Process = core.ProcessIdentity{PID: 1, Namespace: namespace}
	}
	p, _ := f.run.team.Agent(a.ID)
	l, err := p.LockAgent()
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Save(a); err != nil {
		t.Fatal(err)
	}
	l.Close()
	f.env["GANG_AGENT_ID"] = string(a.ID)
	f.env["GANG_AGENT_NONCE"] = "secret"
	f.env["TMUX_PANE"] = a.Pane
	f.cmd.paneBackend = identityFixture{f.input, true, false, nil, nil}
	sandboxed, err := f.cmd.runtime()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sandboxed.observedAgent(); err != nil {
		t.Fatal(err)
	}
	if limited, err := sandboxed.processLimited(a); err != nil || !limited {
		t.Fatalf("sandboxed command not marked: %v %v", limited, err)
	}
	return f, a
}

func TestProcessUnavailableClearsOnlyAfterOwnVerifiedCommand(t *testing.T) {
	namespace := ownNamespace(t)
	for _, tc := range []struct {
		name, namespace string
		background      bool
		cleared         bool
	}{
		{name: "identity readable", namespace: namespace, cleared: true},
		{name: "identity recorded from a sandbox", namespace: "", cleared: false},
		{name: "identity from another namespace", namespace: "pid:[1]", cleared: false},
		{name: "harness not in the foreground", namespace: namespace, background: true, cleared: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, a := sandboxedVerifiedAgent(t, tc.namespace)
			f.cmd.paneBackend = identityFixture{f.input, true, true, nil, nil}
			if tc.background {
				f.input.foregroundErr = errors.New("harness left the foreground")
			}
			visible, err := f.cmd.runtime()
			if err != nil {
				t.Fatal(err)
			}
			if _, err := visible.observedAgent(); (err != nil) != tc.background {
				t.Fatalf("verified command: %v", err)
			}
			limited, err := visible.processLimited(a)
			if err != nil || limited == tc.cleared {
				t.Fatalf("limited=%v err=%v, want cleared=%v", limited, err, tc.cleared)
			}
			log, err := os.ReadFile(f.run.team.Log)
			if err != nil {
				t.Fatal(err)
			}
			if got := strings.Contains(string(log), `"type":"process_verification_available"`); got != tc.cleared {
				t.Fatalf("recovery event=%v, want %v: %s", got, tc.cleared, log)
			}
		})
	}
}

func TestWatchdogRecoversAfterOutageOfElapsedTimer(t *testing.T) {
	for _, recovery := range []string{"snooze", "scoped tick"} {
		t.Run(recovery, func(t *testing.T) {
			f, s := watchdogFixture(t)
			if err := f.cmd.tick(nil); err != nil {
				t.Fatal(err)
			}
			elapsed := s.armed
			// The transient timer unloads once it fires; its intent file stays.
			s.armed = ""
			f.cmd.newScheduler = func() watchdogScheduler { return unavailableWatchdog{s} }
			if err := f.cmd.tick([]string{"--source", "watchdog", "--watchdog", elapsed}); err != nil {
				t.Fatal(err)
			}
			f.cmd.newScheduler = func() watchdogScheduler { return s }
			f.env["GANG_AGENT_ID"] = "a"
			f.env["TMUX_PANE"] = "%1"
			var err error
			if recovery == "snooze" {
				err = f.cmd.snooze([]string{"--at", "2h"})
			} else {
				err = f.cmd.tick([]string{"--agent", "a"})
			}
			if err != nil || s.armed == "" || f.errOut.Len() != 0 {
				t.Fatalf("not rearmed after recovery: %v armed=%q stderr=%q", err, s.armed, f.errOut)
			}
			if _, err := os.Stat(filepath.Join(f.run.team.Directory, "watchdog-unavailable")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("marker survived recovery: %v", err)
			}
		})
	}
}

func TestSandboxedSweepKeepsRecordedTimer(t *testing.T) {
	f, s := watchdogFixture(t)
	if err := f.cmd.tick(nil); err != nil {
		t.Fatal(err)
	}
	armed := s.armed
	f.cmd.paneBackend = identityFixture{f.input, true, false, nil, nil}
	if err := f.cmd.tick(nil); err != nil {
		t.Fatal(err)
	}
	if s.armed != armed {
		t.Fatalf("sandboxed sweep changed the timer: %q -> %q", armed, s.armed)
	}
	if _, err := os.Stat(filepath.Join(f.run.team.Directory, "watchdog-unavailable")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("armed timer reported unavailable: %v", err)
	}
}

func TestVerifiedCallerSurvivesUnclearableProcessMarker(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	f, a := sandboxedVerifiedAgent(t, ownNamespace(t))
	p, _ := f.run.team.Agent(a.ID)
	if err := os.Chmod(p.Directory, 0500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(p.Directory, 0700) })
	f.cmd.paneBackend = identityFixture{f.input, true, true, nil, nil}
	visible, err := f.cmd.runtime()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := visible.observedAgent(); err != nil {
		t.Fatalf("verified caller refused over an informational marker: %v", err)
	}
	if !strings.Contains(f.errOut.String(), "marker not cleared") {
		t.Fatalf("failed clear was silent: %q", f.errOut)
	}
}

func TestElapsedTimerTickWithoutVisibilityReportsOutage(t *testing.T) {
	f, s := watchdogFixture(t)
	if err := f.cmd.tick(nil); err != nil {
		t.Fatal(err)
	}
	elapsed := s.armed
	// The transient timer unloads once it fires; its intent file stays.
	s.armed = ""
	f.cmd.paneBackend = identityFixture{f.input, true, false, nil, nil}
	if err := f.cmd.tick([]string{"--source", "watchdog", "--watchdog", elapsed}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(f.run.team.Directory, "watchdog-unavailable")); err != nil {
		t.Fatalf("elapsed timer left unreplaced without an outage marker: %v", err)
	}
}

func TestElapsedTimerTickThatLosesTheLockReportsOutage(t *testing.T) {
	f, s := watchdogFixture(t)
	if err := f.cmd.tick(nil); err != nil {
		t.Fatal(err)
	}
	elapsed := s.armed
	// The transient timer unloads once it fires; its intent file stays.
	s.armed = ""
	marker := filepath.Join(f.run.team.Directory, "watchdog-unavailable")
	held, err := os.OpenFile(filepath.Join(f.run.team.Directory, "watchdog.lock"), os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer held.Close()
	if err := syscall.Flock(int(held.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatal(err)
	}
	if err := f.cmd.tick([]string{"--source", "watchdog", "--watchdog", "superseded"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("superseded timer reported an outage: %v", err)
	}
	if err := f.cmd.tick([]string{"--source", "watchdog", "--watchdog", elapsed}); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Flock(int(held.Fd()), syscall.LOCK_UN); err != nil {
		t.Fatal(err)
	}
	// The sweep that held the lock cannot see processes, so it keeps the intent.
	f.cmd.paneBackend = identityFixture{f.input, true, false, nil, nil}
	if err := f.cmd.tick(nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("elapsed timer left unreplaced without an outage marker: %v", err)
	}
	f.cmd.paneBackend = identityFixture{f.input, true, true, nil, nil}
	f.env["GANG_AGENT_ID"] = "a"
	f.env["TMUX_PANE"] = "%1"
	if err := f.cmd.snooze([]string{"--at", "2h"}); err != nil || s.armed == "" {
		t.Fatalf("not rearmed after the lost tick: %v armed=%q", err, s.armed)
	}
}

func TestElapsedTimerTickThatFailsReportsOutage(t *testing.T) {
	f, s := watchdogFixture(t)
	if err := f.cmd.tick(nil); err != nil {
		t.Fatal(err)
	}
	elapsed := s.armed
	s.armed = ""
	s.failure = errors.New("scheduler timed out")
	if err := f.cmd.tick([]string{"--source", "watchdog", "--watchdog", elapsed}); !errors.Is(err, s.failure) {
		t.Fatalf("lost arm error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(f.run.team.Directory, "watchdog-unavailable")); err != nil {
		t.Fatalf("failed rearm left no outage marker: %v", err)
	}
	s.failure = nil
	if err := f.cmd.tick([]string{"--agent", "a"}); err != nil || s.armed == "" {
		t.Fatalf("not rearmed after the failed tick: %v armed=%q", err, s.armed)
	}
}
