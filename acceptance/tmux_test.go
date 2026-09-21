package acceptance

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const fakeHarnessEnvironment = "GANGLINE_ACCEPTANCE_FAKE_HARNESS"

func TestMain(m *testing.M) {
	if os.Getenv(fakeHarnessEnvironment) == "1" {
		os.Exit(runFakeHarness())
	}
	os.Exit(m.Run())
}

func TestTmuxCarriesOneHarnessTurn(t *testing.T) {
	tmux, err := exec.LookPath("tmux")
	if err != nil {
		t.Fatal("tmux is required for acceptance tests")
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatalf("locate test binary: %v", err)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("locate home directory: %v", err)
	}
	base := filepath.Join(home, ".local", "state", "gangline", "acceptance")
	if err := os.MkdirAll(base, 0o700); err != nil {
		t.Fatalf("create acceptance state root: %v", err)
	}
	runRoot, err := os.MkdirTemp(base, "run-")
	if err != nil {
		t.Fatalf("create acceptance state: %v", err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(runRoot); err != nil {
			t.Errorf("remove acceptance state: %v", err)
		}
	})

	runner := tmuxRunner{
		binary: tmux,
		socket: filepath.Join(runRoot, "tmux.sock"),
		env:    withoutEnvironment(os.Environ(), "TMUX", "TMUX_PANE"),
	}
	const session = "gangline-acceptance"
	if output, err := runner.run("new-session", "-d", "-s", session); err != nil {
		t.Fatalf("create private tmux session: %v\n%s", err, output)
	}
	t.Cleanup(func() {
		if _, err := runner.run("kill-session", "-t", session); err != nil {
			t.Errorf("remove private tmux session: %v", err)
		}
	})

	listed, err := runner.run("list-sessions", "-F", "#{session_name}")
	if err != nil {
		t.Fatalf("list private tmux sessions: %v\n%s", err, listed)
	}
	if strings.TrimSpace(listed) != session {
		t.Fatalf("private tmux socket contains sessions %q, want only %q", strings.TrimSpace(listed), session)
	}
	pane, err := runner.run("list-panes", "-t", session, "-F", "#{pane_id}")
	if err != nil {
		t.Fatalf("locate private tmux pane: %v\n%s", err, pane)
	}
	pane = strings.TrimSpace(pane)
	if pane == "" || strings.Contains(pane, "\n") {
		t.Fatalf("private tmux session has unexpected panes %q", pane)
	}

	ready := "fake-ready"
	received := "fake-received"
	for key, value := range map[string]string{
		fakeHarnessEnvironment:            "1",
		"GANGLINE_ACCEPTANCE_TMUX":        tmux,
		"GANGLINE_ACCEPTANCE_TMUX_SOCKET": runner.socket,
		"GANGLINE_ACCEPTANCE_READY":       ready,
		"GANGLINE_ACCEPTANCE_RECEIVED":    received,
	} {
		if output, err := runner.run("set-environment", "-t", session, key, value); err != nil {
			t.Fatalf("set fake harness environment: %v\n%s", err, output)
		}
	}
	command := "exec " + shellQuote(executable)
	if output, err := runner.run("respawn-pane", "-k", "-t", pane, command); err != nil {
		t.Fatalf("start fake harness: %v\n%s", err, output)
	}
	if output, err := runner.run("wait-for", ready); err != nil {
		t.Fatalf("wait for fake harness readiness: %v\n%s", err, output)
	}
	if output, err := runner.run("send-keys", "-t", pane, "-l", "hello from gangline"); err != nil {
		t.Fatalf("write fake harness composer: %v\n%s", err, output)
	}
	if output, err := runner.run("send-keys", "-t", pane, "Enter"); err != nil {
		t.Fatalf("submit fake harness composer: %v\n%s", err, output)
	}
	if output, err := runner.run("wait-for", received); err != nil {
		t.Fatalf("wait for fake harness turn: %v\n%s", err, output)
	}
	screen, err := runner.run("capture-pane", "-p", "-t", pane)
	if err != nil {
		t.Fatalf("capture fake harness screen: %v\n%s", err, screen)
	}
	if !strings.Contains(screen, "received: hello from gangline") {
		t.Fatalf("captured screen does not contain delivered turn:\n%s", screen)
	}
}

type tmuxRunner struct {
	binary string
	socket string
	env    []string
}

func (runner tmuxRunner) run(arguments ...string) (string, error) {
	arguments = append([]string{"-S", runner.socket, "-f", "/dev/null"}, arguments...)
	command := exec.Command(runner.binary, arguments...)
	command.Env = runner.env
	output, err := command.CombinedOutput()
	return string(output), err
}

func runFakeHarness() int {
	if err := signal(os.Getenv("GANGLINE_ACCEPTANCE_READY")); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Print("fake> ")
	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		return 1
	}
	fmt.Printf("received: %s\n", scanner.Text())
	if err := signal(os.Getenv("GANGLINE_ACCEPTANCE_RECEIVED")); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if !scanner.Scan() && scanner.Err() != nil {
		return 1
	}
	return 0
}

func signal(channel string) error {
	command := exec.Command(
		os.Getenv("GANGLINE_ACCEPTANCE_TMUX"),
		"-S", os.Getenv("GANGLINE_ACCEPTANCE_TMUX_SOCKET"),
		"wait-for", "-S", channel,
	)
	if output, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("signal %s: %w: %s", channel, err, output)
	}
	return nil
}

func withoutEnvironment(environment []string, names ...string) []string {
	blocked := make(map[string]bool, len(names))
	for _, name := range names {
		blocked[name] = true
	}
	result := make([]string, 0, len(environment))
	for _, entry := range environment {
		name, _, _ := strings.Cut(entry, "=")
		if !blocked[name] {
			result = append(result, entry)
		}
	}
	return result
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}
