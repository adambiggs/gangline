package acceptance

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// A pane that cannot hold itself closes without starting the native CLI. The
// hitch that finds it gone names what the hold printed and its exit status,
// whether the pane closed before the hitch registered it or after.
func TestHitchNamesWhyThePaneHoldFailed(t *testing.T) {
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
	const failure = "pane hold failed: hold refused: wrapper sentinel; exit status 1"
	for _, refused := range []string{"before-registration", "after-registration"} {
		t.Run(refused, func(t *testing.T) {
			const session = "gangline-hold-failure-acceptance"
			team := filepath.Join(root, refused)
			socket := filepath.Join(team, "tmux.sock")
			if err := os.Mkdir(team, 0o700); err != nil {
				t.Fatal(err)
			}
			// The wrapper refuses the hold a pane sets on itself. It orders that
			// refusal against the hitch's registration read, which lists every
			// pane and then resolves the session: before-registration keeps the
			// first listing back until the unheld pane has closed, and
			// after-registration keeps the refusal back until the listing has
			// returned. Each wait is bounded so a missed signal fails the test.
			wrapper := filepath.Join(team, "tmux")
			if err := os.WriteFile(wrapper, []byte(`#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
order='`+refused+`' socket='`+socket+`' listed='`+filepath.Join(team, "listed")+`' bounded='`+exe+`'
case " $* " in
*" remain-on-exit on "*)
	[ "$order" = after-registration ] && GANGLINE_ACCEPTANCE_BOUNDED_WAIT=registered "$bounded" "$socket"
	echo "hold refused: wrapper sentinel" >&2
	exit 1;;
esac
case "$order $3 $4" in
"before-registration list-panes -a")
	if [ ! -e "$listed" ]; then
		: >"$listed"
		GANGLINE_ACCEPTANCE_BOUNDED_WAIT=closed "$bounded" "$socket"
	fi;;
"after-registration display-message -p")
	tmux -S "$socket" wait-for -S registered;;
esac
exec tmux "$@"
`), 0o700); err != nil {
				t.Fatal(err)
			}
			environment := append(withoutEnvironment(os.Environ(), "TMUX", "TMUX_PANE", "GANG_CONFIG_DIR", "GANG_SESSION", "GANG_STATE_ROOT", "GANG_TMUX_SOCKET", "GANG_COLLARS", "GANG_COLLAR", "GANGLINE_HITCH_ID", "GANG_AGENT_ID", "GANG_AGENT_NONCE", "GANG_AGENT_TOKEN"),
				"GANG_SESSION="+session, "GANG_CONFIG_DIR="+filepath.Join(team, "config"), "GANG_STATE_ROOT="+filepath.Join(team, "state"), "GANG_TMUX_SOCKET="+socket, "GANG_COLLARS="+collars, "GANG_COLLAR=acceptance",
				"GANGLINE_ACCEPTANCE_CMD_HARNESS=1", "GANG_TMUX="+wrapper)
			runner := tmuxRunner{binary: "tmux", socket: socket, env: environment}
			t.Cleanup(func() { _, _ = runner.run("kill-session", "-t", "="+session) })
			if out, err := runner.run("new-session", "-d", "-s", session, "-n", "control"); err != nil {
				t.Fatalf("private session: %v %s", err, out)
			}
			if out, err := runner.run("set-hook", "-g", "pane-exited", "wait-for -S closed"); err != nil {
				t.Fatalf("pane exit hook: %v %s", err, out)
			}
			gang := func(args ...string) (string, error) {
				cmd := exec.Command(binary, args...)
				cmd.Dir = repo
				cmd.Env = environment
				out, err := cmd.CombinedOutput()
				return string(out), err
			}
			out, err := gang("hitch", "worker")
			if err == nil || !strings.Contains(out, failure) {
				t.Fatalf("hitch with a refused hold: %v\n%s", err, out)
			}
			// The pane was gone when the hitch read its registration only when
			// the refusal came first.
			if strings.Contains(out, "created pane") != (refused == "before-registration") {
				t.Fatalf("hitch refused %s: %s", refused, out)
			}
			failed, err := gang("log", "--type", "hitch_failed")
			if err != nil || !strings.Contains(failed, failure) {
				t.Fatalf("hitch_failed reason: %v\n%s", err, failed)
			}
			roster, err := gang("roster")
			if err != nil || !strings.Contains(roster, "failed") || !strings.Contains(roster, "wrapper sentinel") {
				t.Fatalf("roster after refused hold: %v\n%s", err, roster)
			}
			assertPanes(t, runner, "control")
			if out, err := gang("drop", "worker"); err != nil {
				t.Fatalf("drop after refused hold: %v\n%s", err, out)
			}
		})
	}
}
