package main

import (
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"testing"
	"time"
)

func TestProcessWatchdogLauncher(t *testing.T) {
	dir := t.TempDir()
	unit := "gangline-test"
	exe := filepath.Join(dir, "gang = ' \" & $()")
	if err := os.WriteFile(exe, []byte("#!/bin/sh\nprintf '%s\\n' \"$@\" \"$GANG_SESSION\" \"${TMUX-unset}\" \"${GANG_COLLARS-unset}\" \"$(pwd)\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMUX", "wrong-socket")
	t.Setenv("GANG_COLLARS", "/stale/collars")
	env := map[string]string{"GANG_SESSION": "team & <friends>", "GANG_STATE_ROOT": "/state with spaces", "PATH": os.Getenv("PATH"), "GANG_COLLARS": ""}
	// A zero delay exercises the real launcher; output waits on the pipe
	// closing, not a sleep or a deadline.
	child := processWatchdog{}.command(unit, exe, env)
	if script := (processWatchdog{delay: 7 * time.Second}).command(unit, exe, env).Args[2]; !strings.HasPrefix(script, "sleep 7 && ") {
		t.Fatalf("timer script: %q", script)
	}
	if !child.SysProcAttr.Setsid {
		t.Fatal("timer shares the arming pane's session")
	}
	out, err := child.CombinedOutput()
	want := "tick\n--source\nwatchdog\n--watchdog\n" + unit + "\n" + env["GANG_SESSION"] + "\nunset\nunset\n/\n"
	if err != nil || string(out) != want {
		t.Fatalf("launcher: output=%q, error=%v", out, err)
	}
}

func TestLinuxWithoutSystemdUsesProcessWatchdog(t *testing.T) {
	if goruntime.GOOS != "linux" {
		t.Skip("Linux scheduler selection")
	}
	t.Setenv("PATH", t.TempDir())
	scheduler, ok := (command{}).watchdogScheduler(t.TempDir()).(processWatchdog)
	if !ok || scheduler.delay != watchdogTimeout {
		t.Fatalf("scheduler: %#v", scheduler)
	}
}
