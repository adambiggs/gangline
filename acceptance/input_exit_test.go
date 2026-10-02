package acceptance

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// A pane holds its native exit until the hitch returns, so a native CLI that
// ends on its startup input fails the agent with its exit status and last
// output, and the hitch removes the pane.
func TestHitchReportsNativeExitOnStartupInput(t *testing.T) {
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
	const session, failure = "gangline-input-exit-acceptance", "error: input-exit sentinel"
	socket := filepath.Join(root, "tmux.sock")
	environment := append(withoutEnvironment(os.Environ(), "TMUX", "TMUX_PANE", "GANG_CONFIG_DIR", "GANG_SESSION", "GANG_STATE_ROOT", "GANG_TMUX_SOCKET", "GANG_COLLARS", "GANG_COLLAR", "GANGLINE_HITCH_ID", "GANG_AGENT_ID", "GANG_AGENT_NONCE", "GANG_AGENT_TOKEN"),
		"GANG_SESSION="+session, "GANG_CONFIG_DIR="+filepath.Join(root, "config"), "GANG_STATE_ROOT="+filepath.Join(root, "state"), "GANG_TMUX_SOCKET="+socket, "GANG_COLLARS="+collars, "GANG_COLLAR=acceptance",
		"GANGLINE_ACCEPTANCE_CMD_HARNESS=1", "GANGLINE_ACCEPTANCE_INPUT_EXIT="+failure,
		"GANGLINE_ACCEPTANCE_TMUX=tmux", "GANGLINE_ACCEPTANCE_TMUX_SOCKET="+socket, "GANGLINE_ACCEPTANCE_LEDGER="+filepath.Join(root, "received"), "GANGLINE_ACCEPTANCE_ARGV_LEDGER="+filepath.Join(root, "argv"))
	runner := tmuxRunner{binary: "tmux", socket: socket, env: environment}
	t.Cleanup(func() { _, _ = runner.run("kill-session", "-t", "="+session) })
	if out, err := runner.run("new-session", "-d", "-s", session, "-n", "control"); err != nil {
		t.Fatalf("private session: %v %s", err, out)
	}
	gang := func(args ...string) (string, error) {
		cmd := exec.Command(binary, args...)
		cmd.Dir = repo
		cmd.Env = environment
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	const exit = "native process exited with status 7"
	out, err := gang("hitch", "worker")
	if err == nil || !strings.Contains(out, exit) || !strings.Contains(out, failure) {
		t.Fatalf("hitch of a native that exits on its startup input: %v\n%s", err, out)
	}
	failed, err := gang("log", "--type", "hitch_failed")
	if err != nil || !strings.Contains(failed, exit) || !strings.Contains(failed, failure) {
		t.Fatalf("hitch_failed: %v\n%s", err, failed)
	}
	roster, err := gang("roster")
	if err != nil || !strings.Contains(roster, "failed") || !strings.Contains(roster, exit) {
		t.Fatalf("roster: %v\n%s", err, roster)
	}
	assertPanes(t, runner, "control")
	if out, err := gang("drop", "worker"); err != nil {
		t.Fatalf("drop: %v\n%s", err, out)
	}
}
