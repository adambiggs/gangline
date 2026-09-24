package acceptance

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/adambiggs/gangline/store"
)

func TestAdoptOwnsExistingPrivatePane(t *testing.T) {
	root, err := os.MkdirTemp(os.TempDir(), "adopt-")
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
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build gang: %v\n%s", err, output)
	}
	const session = "adopt-acceptance"
	socket := filepath.Join(root, "tmux.sock")
	environment := append(withoutEnvironment(os.Environ(), "TMUX", "TMUX_PANE", "GANG_SESSION", "GANG_STATE_ROOT", "GANG_CONFIG_DIR", "GANG_TMUX_SOCKET", "GANG_COLLAR"),
		"GANG_SESSION="+session, "GANG_STATE_ROOT="+filepath.Join(root, "state"), "GANG_CONFIG_DIR="+filepath.Join(root, "config"), "GANG_TMUX_SOCKET="+socket, "GANG_COLLAR=codex")
	runner := tmuxRunner{binary: "tmux", socket: socket, env: environment}
	if output, err := runner.run("new-session", "-d", "-s", session, "-n", "control"); err != nil {
		t.Fatalf("create private session: %v\n%s", err, output)
	}
	t.Cleanup(func() {
		if output, err := runner.run("kill-session", "-t", session); err != nil {
			t.Errorf("remove private session: %v\n%s", err, output)
		}
	})
	if listed, err := runner.run("list-sessions", "-F", "#{session_name}"); err != nil || strings.TrimSpace(listed) != session {
		t.Fatalf("private server sessions = %q: %v", listed, err)
	}
	if output, err := runner.run("new-window", "-d", "-t", session, "-n", "candidate", "cat"); err != nil {
		t.Fatalf("create candidate pane: %v\n%s", err, output)
	}
	pane := strings.TrimSpace(mustTmux(t, runner, "list-panes", "-t", session+":candidate", "-F", "#{pane_id}"))
	pid, err := strconv.Atoi(strings.TrimSpace(mustTmux(t, runner, "display-message", "-p", "-t", pane, "#{pane_pid}")))
	if err != nil || pid <= 0 {
		t.Fatalf("candidate pane pid = %d: %v", pid, err)
	}
	runGang := func(env []string, args ...string) (string, int) {
		t.Helper()
		command := exec.Command(binary, args...)
		command.Dir, command.Env = repo, env
		output, err := command.CombinedOutput()
		if err == nil {
			return string(output), 0
		}
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return string(output), exit.ExitCode()
		}
		return fmt.Sprintf("%v: %s", err, output), -1
	}
	t.Cleanup(func() {
		teamDirectory := filepath.Join(root, "state", "teams", session)
		if _, err := os.Stat(teamDirectory); errors.Is(err, os.ErrNotExist) {
			return
		} else if err != nil {
			t.Errorf("inspect private team for cleanup: %v", err)
			return
		}
		if output, status := runGang(environment, "down", session); status != 0 {
			t.Errorf("remove private team status=%d: %s", status, output)
		}
	})
	if output, status := runGang(append(environment, "TMUX_PANE="+pane), "adopt", "adopted", "-c", "codex"); status != 0 {
		t.Fatalf("adopt status=%d: %s", status, output)
	}
	team, err := (store.Paths{Root: filepath.Join(root, "state")}).Team(session)
	if err != nil {
		t.Fatal(err)
	}
	id, err := team.ResolveName("adopted")
	if err != nil {
		t.Fatal(err)
	}
	paths, err := team.Agent(id)
	if err != nil {
		t.Fatal(err)
	}
	registered, err := paths.Read()
	if err != nil {
		t.Fatal(err)
	}
	if registered.Pane != pane || registered.Process.PID != pid || registered.Process.Started == "" || registered.Process.BootID == "" {
		t.Fatalf("adopted pane identity = %+v, want pane %s pid %d", registered, pane, pid)
	}
	if output, status := runGang(environment, "drop", "adopted"); status != 0 {
		t.Fatalf("drop adopted status=%d: %s", status, output)
	}
	panes := mustTmux(t, runner, "list-panes", "-s", "-t", session, "-F", "#{pane_id}")
	if strings.Contains(panes, pane+"\n") || strings.TrimSpace(panes) == "" {
		t.Fatalf("drop did not isolate adopted pane: %q", panes)
	}
}

func mustTmux(t *testing.T, runner tmuxRunner, args ...string) string {
	t.Helper()
	output, err := runner.run(args...)
	if err != nil {
		t.Fatalf("private tmux %v: %v\n%s", args, err, output)
	}
	return output
}
