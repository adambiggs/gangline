//go:build linux

package tmux

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"
)

func TestProcStatPreservesSubsecondIdentityAndParentheses(t *testing.T) {
	parse := func(start string) processRecord {
		t.Helper()
		fields := append([]string{"S", "100"}, strings.Fields("0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0")...)
		fields = append(fields, start)
		record, err := parseProcStat("200 (name with ) parens) " + strings.Join(fields, " "))
		if err != nil {
			t.Fatal(err)
		}
		return record
	}
	before, after := parse("123401"), parse("123402")
	if before.PID != 200 || before.ParentPID != 100 || before.started == after.started {
		t.Fatalf("precise process identity lost: before=%+v after=%+v", before, after)
	}
}

func TestLinuxPinnedHandleSurvivesProcessExit(t *testing.T) {
	command := exec.Command("sh", "-c", "IFS= read -r value")
	input, err := command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = input.Close(); _ = command.Wait() })
	observation, err := observeProcess(command.Process.Pid)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = observation.close() })
	identity, err := pinObservedProcess(observation.record, observation.read, openProcessHandle)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = closeOwnedProcesses([]processIdentity{identity}) })
	native := identity.handle.(*linuxProcessHandle)
	var pinnedFD uintptr
	if err := native.process.WithHandle(func(fd uintptr) { pinnedFD = fd }); err != nil {
		t.Fatal(err)
	}
	if err := input.Close(); err != nil {
		t.Fatal(err)
	}
	_ = command.Wait() // EOF, then Wait, is the process-exit barrier.
	if err := signalProcesses([]processIdentity{identity}, syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	if err := identity.handle.wait(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := native.process.WithHandle(func(fd uintptr) {
		if fd != pinnedFD {
			t.Errorf("wait changed pidfd from %d to %d", pinnedFD, fd)
		}
	}); err != nil {
		t.Fatal(err)
	}
	info, err := os.ReadFile(fmt.Sprintf("/proc/self/fdinfo/%d", pinnedFD))
	if err != nil || !strings.Contains(string(info), "Pid:\t-1") {
		t.Fatalf("retained pidfd does not refer to exited process: %q, %v", info, err)
	}
}

func TestBackendKillClosesHandlesWhenTmuxRefuses(t *testing.T) {
	command := exec.Command("sh", "-c", "IFS= read -r value")
	input, err := command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = input.Close(); _ = command.Wait() })
	pid := command.Process.Pid
	countHandles := func() int {
		t.Helper()
		entries, err := os.ReadDir("/proc/self/fdinfo")
		if err != nil {
			t.Fatal(err)
		}
		count := 0
		for _, entry := range entries {
			data, err := os.ReadFile("/proc/self/fdinfo/" + entry.Name())
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(data), fmt.Sprintf("Pid:\t%d\n", pid)) {
				count++
			}
		}
		return count
	}
	before := countHandles()
	binary := t.TempDir() + "/fake-tmux"
	script := fmt.Sprintf("#!/bin/sh\ncase \"$1\" in\ndisplay-message) echo %d 0;;\nkill-window) echo fixture-refusal >&2; exit 1;;\n*) exit 2;;\nesac\n", pid)
	if err := os.WriteFile(binary, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	backend, err := New(Config{Binary: binary, Session: "identity-cleanup-fixture"})
	if err != nil {
		t.Fatal(err)
	}
	if err := backend.Kill(context.Background(), "%1"); err == nil || !strings.Contains(err.Error(), "fixture-refusal") {
		t.Fatalf("expected failure after handle acquisition, got %v", err)
	}
	if after := countHandles(); after != before {
		t.Fatalf("tmux failure leaked pinned handles: before=%d after=%d", before, after)
	}
}

func TestLinuxSnapshotReadCannotFollowExitedProcess(t *testing.T) {
	command := exec.Command("sh", "-c", "IFS= read -r value")
	input, err := command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = input.Close(); _ = command.Wait() })
	observation, err := observeProcess(command.Process.Pid)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = observation.close() })
	if err := input.Close(); err != nil {
		t.Fatal(err)
	}
	_ = command.Wait() // Reaping the original is the snapshot/acquisition barrier.
	opened := false
	_, err = pinObservedProcess(observation.record, observation.read, func(processRecord) (processHandle, error) {
		opened = true
		return &fakeProcessHandle{}, nil
	})
	if !processGone(err) || opened {
		t.Fatalf("original procfs snapshot followed a PID after exit: err=%v opened=%v", err, opened)
	}
}
