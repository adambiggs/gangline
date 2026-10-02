package acceptance

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// A native CLI that exits before its startup screen fails the hitch with its
// own output, records that output as the failure reason, and leaves no pane.
func TestHitchReportsNativeBootExit(t *testing.T) {
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
	// The startup capture is the one tmux call with both capture-pane and -e.
	// This wrapper fails it, or lets the native CLI exit before it runs, so
	// each case reaches its path without a clock.
	wrapper := filepath.Join(root, "tmux")
	if err := os.WriteFile(wrapper, []byte(`#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
capture=0 color=0
for argument; do
	case "$argument" in capture-pane) capture=1;; -e) color=1;; esac
done
if [ "$capture$color" = 11 ]; then
	case "$GANGLINE_ACCEPTANCE_CAPTURE" in
	fail) echo 'capture refused' >&2; exit 1;;
	exit) tmux -S "$GANG_TMUX_SOCKET" wait-for -S boot-exit || exit 1;;
	# The pipe reaches EOF when the native CLI exits, and run-shell makes tmux
	# reap it, so only the release after the failed startup can see the exit.
	fail-exit) tmux -S "$GANG_TMUX_SOCKET" wait-for -S boot-exit || exit 1
		cat "$GANGLINE_ACCEPTANCE_EXIT_PIPE" >/dev/null
		tmux -S "$GANG_TMUX_SOCKET" run-shell true
		echo 'capture refused' >&2; exit 1;;
	esac
fi
exec tmux "$@"
`), 0o700); err != nil {
		t.Fatal(err)
	}
	const failure = "error: unknown model boot-exit-sentinel"
	for _, tc := range []struct {
		name     string
		existing bool
		// capture is the wrapper's mode for the startup capture.
		capture string
		// exits is false when the native CLI stays up and only startup fails.
		exits bool
		// blocked shows a startup prompt, and the native CLI exits only after
		// the hitch returns.
		blocked bool
	}{
		{name: "spawn", existing: true, exits: true},
		{name: "create", exits: true},
		{name: "exit-after-spawned", existing: true, capture: "exit", exits: true},
		{name: "startup-error", existing: true, capture: "fail"},
		{name: "exit-after-startup-error", existing: true, capture: "fail-exit", exits: true},
		{name: "exit-at-blocked-startup", existing: true, exits: true, blocked: true},
	} {
		name, existing := tc.name, tc.existing
		t.Run(name, func(t *testing.T) {
			const session = "gangline-boot-exit-acceptance"
			team := filepath.Join(root, name)
			socket := filepath.Join(team, "tmux.sock")
			if err := os.Mkdir(team, 0o700); err != nil {
				t.Fatal(err)
			}
			environment := append(withoutEnvironment(os.Environ(), "TMUX", "TMUX_PANE", "GANG_CONFIG_DIR", "GANG_SESSION", "GANG_STATE_ROOT", "GANG_TMUX_SOCKET", "GANG_COLLARS", "GANG_COLLAR", "GANGLINE_HITCH_ID", "GANG_AGENT_ID", "GANG_AGENT_NONCE", "GANG_AGENT_TOKEN"),
				"GANG_SESSION="+session, "GANG_CONFIG_DIR="+filepath.Join(team, "config"), "GANG_STATE_ROOT="+filepath.Join(team, "state"), "GANG_TMUX_SOCKET="+socket, "GANG_COLLARS="+collars, "GANG_COLLAR=acceptance",
				"GANGLINE_ACCEPTANCE_CMD_HARNESS=1", "GANG_TMUX="+wrapper, "GANGLINE_ACCEPTANCE_CAPTURE="+tc.capture)
			if tc.exits {
				environment = append(environment, "GANGLINE_ACCEPTANCE_BOOT_EXIT="+failure)
			}
			if tc.capture == "fail-exit" {
				pipe := filepath.Join(team, "exit-pipe")
				if err := syscall.Mkfifo(pipe, 0o600); err != nil {
					t.Fatal(err)
				}
				// Bounded so a native process that never opens the pipe cannot hold
				// the wrapper's read, and with it the hitch, forever: a writer that
				// opens and closes ends the read.
				bound := time.AfterFunc(time.Minute, func() {
					if writer, err := os.OpenFile(pipe, os.O_WRONLY|syscall.O_NONBLOCK, 0); err == nil {
						_ = writer.Close()
					}
				})
				t.Cleanup(func() { bound.Stop() })
				environment = append(environment, "GANGLINE_ACCEPTANCE_BOOT_EXIT_AFTER=boot-exit", "GANGLINE_ACCEPTANCE_EXIT_PIPE="+pipe)
			} else if tc.blocked {
				pipe := filepath.Join(team, "exit-pipe")
				if err := syscall.Mkfifo(pipe, 0o600); err != nil {
					t.Fatal(err)
				}
				environment = append(environment, "GANGLINE_ACCEPTANCE_BOOT_BLOCKED=1", "GANGLINE_ACCEPTANCE_BOOT_EXIT_AFTER=boot-exit", "GANGLINE_ACCEPTANCE_EXIT_PIPE="+pipe)
			} else if tc.capture == "exit" {
				environment = append(environment, "GANGLINE_ACCEPTANCE_BOOT_EXIT_AFTER=boot-exit")
			} else {
				environment = append(environment, "GANGLINE_ACCEPTANCE_TMUX=tmux", "GANGLINE_ACCEPTANCE_TMUX_SOCKET="+socket, "GANGLINE_ACCEPTANCE_LEDGER="+filepath.Join(team, "received"), "GANGLINE_ACCEPTANCE_ARGV_LEDGER="+filepath.Join(team, "argv"))
			}
			runner := tmuxRunner{binary: "tmux", socket: socket, env: environment}
			t.Cleanup(func() { _, _ = runner.run("kill-session", "-t", "="+session) })
			if existing {
				if out, err := runner.run("new-session", "-d", "-s", session, "-n", "control"); err != nil {
					t.Fatalf("private session: %v %s", err, out)
				}
			}
			gang := func(args ...string) (string, error) {
				cmd := exec.Command(binary, args...)
				cmd.Dir = repo
				cmd.Env = environment
				out, err := cmd.CombinedOutput()
				return string(out), err
			}
			out, err := gang("hitch", "worker")
			if err == nil {
				t.Fatalf("hitch succeeded:\n%s", out)
			}
			if tc.blocked {
				if !strings.Contains(out, "resolve native prompts") {
					t.Fatalf("hitch did not report the blocked startup:\n%s", out)
				}
				// The operator's answer ends the native CLI after the hitch
				// returned; the pipe's EOF is the exit barrier.
				exited := make(chan error, 1)
				go func() {
					pipe, err := os.Open(filepath.Join(team, "exit-pipe"))
					if err != nil {
						exited <- err
						return
					}
					defer pipe.Close()
					_, err = io.Copy(io.Discard, pipe)
					exited <- err
				}()
				if out, err := runner.run("wait-for", "-S", "boot-exit"); err != nil {
					t.Fatalf("answer startup prompt: %v %s", err, out)
				}
				select {
				case err := <-exited:
					if err != nil {
						t.Fatalf("await native exit: %v", err)
					}
				case <-time.After(time.Minute):
					t.Fatal("native CLI did not exit")
				}
				if out, err := runner.run("run-shell", "true"); err != nil {
					t.Fatalf("reap native process: %v %s", err, out)
				}
				if out, err := gang("tick", "--agent", "worker"); err != nil {
					t.Fatalf("tick after native exit: %v\n%s", err, out)
				}
				out, err := gang("roster")
				if err != nil || !strings.Contains(out, "failed") || !strings.Contains(out, failure) {
					t.Fatalf("roster lacks the native exit: %v\n%s", err, out)
				}
				log, err := gang("log", "--type", "hitch_failed")
				if err != nil || !strings.Contains(log, failure) {
					t.Fatalf("hitch_failed reason lacks the native output: %v\n%s", err, log)
				}
				return
			}
			if !tc.exits {
				if !strings.Contains(out, "capture refused") {
					t.Fatalf("hitch error lacks the startup failure:\n%s", out)
				}
				panes, err := runner.run("list-panes", "-a", "-F", "#{pane_id} #{window_name}")
				if err != nil {
					t.Fatalf("list panes: %v %s", err, panes)
				}
				kept := 0
				for _, line := range strings.Split(strings.TrimSpace(panes), "\n") {
					id, window, _ := strings.Cut(line, " ")
					if window == "control" {
						continue
					}
					kept++
					if held, err := runner.run("show-options", "-p", "-v", "-t", id, "remain-on-exit"); err != nil || strings.TrimSpace(held) == "on" {
						t.Fatalf("kept pane %s still holds its exit: %v %q", id, err, held)
					}
				}
				if kept != 1 {
					t.Fatalf("panes after startup failure: %q", panes)
				}
				return
			}
			if tc.capture == "exit" {
				if spawned, err := gang("log", "--type", "hitch_spawned"); err != nil || strings.TrimSpace(spawned) == "" {
					t.Fatalf("native exit landed before hitch_spawned: %v %q", err, spawned)
				}
			}
			if !strings.Contains(out, failure) {
				t.Fatalf("hitch error lacks the native output:\n%s", out)
			}
			log, err := gang("log", "--type", "hitch_failed")
			if err != nil || !strings.Contains(log, failure) {
				t.Fatalf("hitch_failed reason lacks the native output: %v\n%s", err, log)
			}
			panes, err := runner.run("list-panes", "-a", "-F", "#{window_name}")
			if existing {
				if err != nil || strings.TrimSpace(panes) != "control" {
					t.Fatalf("panes after failed hitch: %v %q", err, panes)
				}
			} else if err == nil {
				t.Fatalf("server kept panes after failed hitch: %q", panes)
			}
			// The failed record never fails a tick of its agent.
			if out, err := gang("tick", "--agent", "worker"); err != nil {
				t.Fatalf("tick after failed hitch: %v\n%s", err, out)
			}
		})
	}
}
