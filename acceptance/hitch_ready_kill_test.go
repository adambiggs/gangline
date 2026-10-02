package acceptance

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// A pane holds its native exit until the hitch returns, and the record says so
// until the hold is released. So a hitch killed after it recorded the agent
// ready leaves a hold that the next tick ends.
func TestHitchKilledAtReadinessLeavesNoHeldPane(t *testing.T) {
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
	// The wrapper names two tmux calls: a release of the pane hold, by the hitch
	// or by a tick, and the window rename that follows the ready record. It
	// lists each one it sees, and kills gang, its parent, at the chosen one,
	// failing the call. Its files sit beside the state root, so the native
	// CLI's hooks reach them as the hitch does.
	wrapper := filepath.Join(root, "tmux")
	if err := os.WriteFile(wrapper, []byte(`#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
point= unset=0 hold=0
for argument; do
	case "$argument" in
	*rename-window*'\~worker\~'*) point=ready;;
	*'"-u"'*'"remain-on-exit"'*) point=release;;
	-u) unset=1;;
	remain-on-exit) hold=1;;
	esac
done
[ "$unset$hold" = 11 ] && point=release
team=${GANG_STATE_ROOT%/*}
if [ -n "$point" ]; then
	echo "$point" >>"$team/points"
	if [ -e "$team/kill-at-$point" ]; then
		kill -s KILL "$PPID"
		exit 1
	fi
fi
exec tmux "$@"
`), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, point := range []string{"release", "ready", "never"} {
		t.Run("killed-"+point, func(t *testing.T) {
			const session = "gangline-ready-kill-acceptance"
			team := filepath.Join(root, point)
			socket := filepath.Join(team, "tmux.sock")
			if err := os.Mkdir(team, 0o700); err != nil {
				t.Fatal(err)
			}
			environment := append(withoutEnvironment(os.Environ(), "TMUX", "TMUX_PANE", "GANG_CONFIG_DIR", "GANG_SESSION", "GANG_STATE_ROOT", "GANG_TMUX_SOCKET", "GANG_COLLARS", "GANG_COLLAR", "GANGLINE_HITCH_ID", "GANG_AGENT_ID", "GANG_AGENT_NONCE", "GANG_AGENT_TOKEN"),
				"GANG_SESSION="+session, "GANG_CONFIG_DIR="+filepath.Join(team, "config"), "GANG_STATE_ROOT="+filepath.Join(team, "state"), "GANG_TMUX_SOCKET="+socket, "GANG_COLLARS="+collars, "GANG_COLLAR=acceptance",
				"GANGLINE_ACCEPTANCE_CMD_HARNESS=1", "GANG_TMUX="+wrapper,
				"GANGLINE_ACCEPTANCE_TMUX=tmux", "GANGLINE_ACCEPTANCE_TMUX_SOCKET="+socket, "GANGLINE_ACCEPTANCE_LEDGER="+filepath.Join(team, "received"), "GANGLINE_ACCEPTANCE_ARGV_LEDGER="+filepath.Join(team, "argv"))
			runner := tmuxRunner{binary: "tmux", socket: socket, env: environment}
			t.Cleanup(func() { _, _ = runner.run("kill-session", "-t", "="+session) })
			if out, err := runner.run("new-session", "-d", "-s", session, "-n", "control"); err != nil {
				t.Fatalf("private session: %v %s", err, out)
			}
			// Output goes to a file, not a pipe: a process the killed gang left
			// behind inherits its descriptors and would hold a pipe open.
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
			points, kill := filepath.Join(team, "points"), filepath.Join(team, "kill-at-"+point)
			if err := os.WriteFile(kill, nil, 0o600); err != nil {
				t.Fatal(err)
			}
			held := func() bool {
				t.Helper()
				records, err := filepath.Glob(filepath.Join(team, "state", "teams", session, "agents", "*", "agent.json"))
				if err != nil || len(records) != 1 {
					t.Fatalf("agent records: %v %q", err, records)
				}
				data, err := os.ReadFile(records[0])
				if err != nil {
					t.Fatal(err)
				}
				var record struct {
					Registration struct {
						Held bool `json:"held"`
					} `json:"registration"`
				}
				if err := json.Unmarshal(data, &record); err != nil {
					t.Fatal(err)
				}
				return record.Registration.Held
			}
			unheld := func() {
				t.Helper()
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
						t.Fatalf("ready agent's pane %s still holds its exit: %v %q", id, err, held)
					}
				}
				if kept != 1 {
					t.Fatalf("agent panes: %q", panes)
				}
				if held() {
					t.Fatal("record says the released pane is held")
				}
			}
			out, state, err := gang(environment, "hitch", "worker")
			if point == "never" {
				if err != nil {
					t.Fatalf("hitch: %v\n%s", err, out)
				}
				seen, err := os.ReadFile(points)
				if err != nil {
					t.Fatal(err)
				}
				// A tick the hitch leaves behind can rename the window again, and
				// the hold is released once, by the hitch or by that tick.
				if !strings.HasPrefix(string(seen), "ready\n") || strings.Count(string(seen), "release") != 1 {
					t.Fatalf("hitch calls = %q, want the ready rename, then one release", seen)
				}
				unheld()
				return
			}
			if status, ok := state.Sys().(syscall.WaitStatus); err == nil || !ok || !status.Signaled() || status.Signal() != syscall.SIGKILL {
				t.Fatalf("hitch did not die by SIGKILL: %v\n%s", err, out)
			}
			if err := os.Remove(kill); err != nil {
				t.Fatal(err)
			}
			ready, _, err := gang(environment, "log", "--type", "hitch_ready")
			if err != nil || strings.TrimSpace(ready) == "" {
				t.Fatalf("hitch_ready before the tick, killed at %s: %v %q", point, err, ready)
			}
			if out, _, err := gang(environment, "tick", "--agent", "worker"); err != nil {
				t.Fatalf("tick after killed hitch: %v\n%s", err, out)
			}
			roster, _, err := gang(environment, "roster")
			if err != nil || !strings.Contains(roster, "active") {
				t.Fatalf("roster after tick: %v\n%s", err, roster)
			}
			unheld()
			if out, _, err := gang(environment, "drop", "worker"); err != nil || strings.Contains(out, "warning") {
				t.Fatalf("drop after killed hitch: %v\n%s", err, out)
			}
			assertPanes(t, runner, "control")
		})
	}
}
