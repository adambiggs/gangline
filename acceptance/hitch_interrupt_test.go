package acceptance

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// A hitch stopped by a signal mid-boot leaves the team usable: a handled
// signal fails the record as interrupted and removes the pane it created; a
// kill leaves the pane recorded, so tick keeps working and drop removes it.
func TestInterruptedHitchLeavesTeamUsable(t *testing.T) {
	root, err := os.MkdirTemp(os.TempDir(), "team-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	repo, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(root, "gang")
	build := exec.Command("go", "build", "-o", binary, "./cmd/gang")
	build.Dir = repo
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	collars := filepath.Join(root, "collars")
	if err := os.Mkdir(collars, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(collars, "acceptance.cue"), []byte(commandAcceptanceCollar(exe)), 0o600); err != nil {
		t.Fatal(err)
	}
	// The wrapper signals gang, its parent, once at each named tmux call: the
	// pane creation, the process-visibility read that follows registration,
	// the startup capture, or the pane removal. It then runs the call, blocks
	// it until gang cancels it, or fails it, so each case reaches its point
	// without a clock.
	wrapper := filepath.Join(root, "tmux")
	if err := os.WriteFile(wrapper, []byte(`#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
point= capture=0 color=0
for argument; do
	case "$argument" in
	new-window) point=create;;
	*'#{socket_path}'*) point=visibility;;
	'kill-pane -t '*) point=removal;;
	capture-pane) capture=1;;
	-e) color=1;;
	esac
done
[ "$capture$color" = 11 ] && point=startup
for step in $GANGLINE_ACCEPTANCE_INTERRUPT; do
	[ "${step%%:*}" = "$point" ] || continue
	[ -e "$GANGLINE_ACCEPTANCE_INTERRUPT_ONCE.$point" ] && break
	mkdir "$GANGLINE_ACCEPTANCE_INTERRUPT_ONCE.$point" || exit 1
	kill -s "$GANGLINE_ACCEPTANCE_INTERRUPT_SIGNAL" "$PPID" || exit 1
	case "${step#*:}" in
	block) exec tmux -S "$GANG_TMUX_SOCKET" wait-for interrupt-released;;
	fail) echo 'interrupted call failed' >&2; exit 1;;
	esac
done
exec tmux "$@"
`), 0o700); err != nil {
		t.Fatal(err)
	}
	const failure = "error: unknown model interrupt-sentinel"
	for _, tc := range []struct {
		name   string
		signal syscall.Signal
		// steps names each call the wrapper signals at and what the call
		// then does: run, block, or fail.
		steps string
		// exits is true when the native CLI exits after gang is killed.
		exits bool
	}{
		{name: "sigint-at-registration", signal: syscall.SIGINT, steps: "visibility:block"},
		{name: "sigterm-at-startup", signal: syscall.SIGTERM, steps: "startup:block"},
		{name: "second-sigint-at-removal", signal: syscall.SIGINT, steps: "visibility:block removal:run"},
		{name: "sigkill-at-registration", signal: syscall.SIGKILL, steps: "visibility:fail"},
		{name: "sigkill-before-create", signal: syscall.SIGKILL, steps: "create:fail"},
		{name: "sigkill-then-native-exit", signal: syscall.SIGKILL, steps: "visibility:fail", exits: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			const session = "gangline-interrupt-acceptance"
			team := filepath.Join(root, tc.name)
			socket := filepath.Join(team, "tmux.sock")
			if err := os.Mkdir(team, 0o700); err != nil {
				t.Fatal(err)
			}
			environment := append(withoutEnvironment(os.Environ(), "TMUX", "TMUX_PANE", "GANG_CONFIG_DIR", "GANG_SESSION", "GANG_STATE_ROOT", "GANG_TMUX_SOCKET", "GANG_COLLARS", "GANG_COLLAR", "GANGLINE_HITCH_ID", "GANG_AGENT_ID", "GANG_AGENT_NONCE", "GANG_AGENT_TOKEN"),
				"GANG_SESSION="+session, "GANG_CONFIG_DIR="+filepath.Join(team, "config"), "GANG_STATE_ROOT="+filepath.Join(team, "state"), "GANG_TMUX_SOCKET="+socket, "GANG_COLLARS="+collars, "GANG_COLLAR=acceptance",
				"GANGLINE_ACCEPTANCE_CMD_HARNESS=1", "GANG_TMUX="+wrapper)
			hitchEnvironment := append(append([]string(nil), environment...),
				"GANGLINE_ACCEPTANCE_INTERRUPT="+tc.steps, "GANGLINE_ACCEPTANCE_INTERRUPT_SIGNAL="+strings.TrimPrefix(signalName(tc.signal), "SIG"), "GANGLINE_ACCEPTANCE_INTERRUPT_ONCE="+filepath.Join(team, "interrupted"))
			// The native CLI takes its mode from the tmux server's environment.
			var native []string
			pipe := filepath.Join(team, "exit-pipe")
			if tc.exits {
				if err := syscall.Mkfifo(pipe, 0o600); err != nil {
					t.Fatal(err)
				}
				native = append(native, "GANGLINE_ACCEPTANCE_BOOT_EXIT="+failure, "GANGLINE_ACCEPTANCE_BOOT_EXIT_AFTER=boot-exit", "GANGLINE_ACCEPTANCE_EXIT_PIPE="+pipe)
			} else {
				// The native CLI never renders its composer, so startup cannot
				// finish before the signal is handled.
				release := filepath.Join(team, "release")
				if err := syscall.Mkfifo(release, 0o600); err != nil {
					t.Fatal(err)
				}
				native = append(native, "GANGLINE_ACCEPTANCE_RELEASE_FIFO="+release, "GANGLINE_ACCEPTANCE_ARGV_LEDGER="+filepath.Join(team, "argv"))
			}
			runner := tmuxRunner{binary: "tmux", socket: socket, env: append(append([]string(nil), environment...), native...)}
			t.Cleanup(func() { _, _ = runner.run("kill-session", "-t", "="+session) })
			if out, err := runner.run("new-session", "-d", "-s", session, "-n", "control"); err != nil {
				t.Fatalf("private session: %v %s", err, out)
			}
			// Output goes to a file, not a pipe: a tmux client left blocked by a
			// killed gang inherits its descriptors and would hold a pipe open.
			gang := func(env []string, args ...string) (string, *os.ProcessState, error) {
				output, err := os.CreateTemp(team, "gang-")
				if err != nil {
					t.Fatal(err)
				}
				defer output.Close()
				cmd := exec.Command(binary, args...)
				cmd.Dir = repo
				cmd.Env = env
				cmd.Stdout, cmd.Stderr = output, output
				err = cmd.Run()
				text, readErr := os.ReadFile(output.Name())
				if readErr != nil {
					t.Fatal(readErr)
				}
				return string(text), cmd.ProcessState, err
			}
			out, state, err := gang(hitchEnvironment, "hitch", "worker")
			// Release a call the wrapper blocked: gang cancels it when it handles
			// the signal, and nothing else ends it when the signal killed gang.
			if strings.Contains(tc.steps, ":block") {
				if released, err := runner.run("wait-for", "-S", "interrupt-released"); err != nil {
					t.Fatalf("release blocked call: %v %s", err, released)
				}
			}
			if err == nil {
				t.Fatalf("hitch succeeded:\n%s", out)
			}
			if tc.signal == syscall.SIGKILL {
				if status, ok := state.Sys().(syscall.WaitStatus); !ok || !status.Signaled() || status.Signal() != syscall.SIGKILL {
					t.Fatalf("hitch did not die by SIGKILL: %v\n%s", err, out)
				}
			} else {
				var exit *exec.ExitError
				if !errors.As(err, &exit) || exit.ExitCode() != 1 || !strings.Contains(out, "hitch interrupted by "+signalName(tc.signal)) {
					t.Fatalf("hitch did not report the interrupt: %v\n%s", err, out)
				}
				failed, _, err := gang(environment, "log", "--type", "hitch_failed")
				if err != nil || !strings.Contains(failed, "hitch interrupted by "+signalName(tc.signal)) {
					t.Fatalf("hitch_failed reason lacks the interrupt: %v\n%s", err, failed)
				}
				assertPanes(t, runner, "control")
			}
			if tc.exits {
				// The native CLI exits once released, and the pipe's EOF plus
				// run-shell show tmux has reaped it while the pane is held.
				bound := time.AfterFunc(time.Minute, func() {
					if writer, err := os.OpenFile(pipe, os.O_WRONLY|syscall.O_NONBLOCK, 0); err == nil {
						_ = writer.Close()
					}
				})
				defer bound.Stop()
				if out, err := runner.run("wait-for", "-S", "boot-exit"); err != nil {
					t.Fatalf("release native exit: %v %s", err, out)
				}
				if _, err := os.ReadFile(pipe); err != nil {
					t.Fatal(err)
				}
				if out, err := runner.run("run-shell", "true"); err != nil {
					t.Fatalf("reap native exit: %v %s", err, out)
				}
			}
			// The targeted tick waits for the agent's lock and probes its
			// record; the bare tick then covers the whole team.
			for _, args := range [][]string{{"tick", "--agent", "worker"}, {"tick"}} {
				if out, _, err := gang(environment, args...); err != nil {
					debug, _, _ := gang(environment, "log")
					t.Fatalf("%v after interrupted hitch: %v\n%s\nteam log:\n%s", args, err, out, debug)
				}
			}
			if tc.exits {
				failed, _, err := gang(environment, "log", "--type", "hitch_failed")
				if err != nil || !strings.Contains(failed, failure) {
					t.Fatalf("hitch_failed reason lacks the native output: %v\n%s", err, failed)
				}
			}
			if tc.signal == syscall.SIGKILL {
				out, _, err := gang(environment, "drop", "worker")
				if err != nil {
					t.Fatalf("drop after killed hitch: %v\n%s", err, out)
				}
				// Every pane and process the hitch recorded is visible here.
				if strings.Contains(out, "warning") {
					t.Fatalf("drop after killed hitch warned:\n%s", out)
				}
				assertPanes(t, runner, "control")
			}
			// A later hitch is not refused: it spawns, and its native CLI's
			// boot exit is the failure it reports.
			for _, name := range []string{"GANGLINE_ACCEPTANCE_BOOT_EXIT_AFTER", "GANGLINE_ACCEPTANCE_EXIT_PIPE", "GANGLINE_ACCEPTANCE_RELEASE_FIFO"} {
				if out, err := runner.run("set-environment", "-gu", name); err != nil {
					t.Fatalf("unset %s: %v %s", name, err, out)
				}
			}
			if out, err := runner.run("set-environment", "-g", "GANGLINE_ACCEPTANCE_BOOT_EXIT", failure); err != nil {
				t.Fatalf("set boot exit: %v %s", err, out)
			}
			out, _, err = gang(environment, "hitch", "second")
			if err == nil || !strings.Contains(out, failure) {
				t.Fatalf("later hitch did not spawn: %v\n%s", err, out)
			}
		})
	}
}

func signalName(signal syscall.Signal) string {
	switch signal {
	case syscall.SIGINT:
		return "SIGINT"
	case syscall.SIGTERM:
		return "SIGTERM"
	case syscall.SIGKILL:
		return "SIGKILL"
	}
	return signal.String()
}

func assertPanes(t *testing.T, runner tmuxRunner, want string) {
	t.Helper()
	panes, err := runner.run("list-panes", "-a", "-F", "#{window_name}")
	if err != nil || strings.TrimSpace(panes) != want {
		t.Fatalf("panes = %q (%v), want %q", panes, err, want)
	}
}
