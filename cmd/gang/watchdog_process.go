package main

import (
	"fmt"
	"os"
	"os/exec"
	"slices"
	"sort"
	"strings"
	"syscall"
	"time"
)

// processWatchdog serves a Linux host without a systemd user manager, such as
// a microVM whose init is not systemd. Its timer is a detached session that
// sleeps out the cadence and then becomes the tick, so it lives as long as the
// host's user processes do, like the team's tmux server.
type processWatchdog struct{ delay time.Duration }

func (s processWatchdog) Arm(unit, executable string, environment map[string]string) error {
	child := s.command(unit, executable, environment)
	if err := child.Start(); err != nil {
		return err
	}
	return child.Process.Release()
}

// Disarm leaves the sleeper to expire. Its tick finds its generation
// superseded or the team gone and exits without work, and a sleeper that has
// become its own tick must not be killed by that tick's re-arm.
func (processWatchdog) Disarm(string) error { return nil }

func (s processWatchdog) command(unit, executable string, environment map[string]string) *exec.Cmd {
	child := exec.Command("/bin/sh", "-c", fmt.Sprintf(`sleep %d && exec "$@"`, int(s.delay/time.Second)), "gangline-watchdog", executable, "tick", "--source", "watchdog", "--watchdog", unit)
	child.Dir = "/"
	// A new session survives the hangup of the pane that armed it. Nil
	// standard streams connect to the null device; the tick records its own
	// failures in the team log.
	child.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	routing := append(append([]string(nil), watchdogUnsetEnvironment...), watchdogEnvironmentKeys...)
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if !slices.Contains(routing, key) {
			child.Env = append(child.Env, entry)
		}
	}
	keys := make([]string, 0, len(environment))
	for key, value := range environment {
		if value != "" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	for _, key := range keys {
		child.Env = append(child.Env, key+"="+environment[key])
	}
	return child
}
