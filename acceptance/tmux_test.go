package acceptance

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const fakeHarnessEnvironment = "GANGLINE_ACCEPTANCE_FAKE_HARNESS"

func TestMain(m *testing.M) {
	if os.Getenv("GANGLINE_ACCEPTANCE_CMD_HARNESS") == "1" {
		os.Exit(runCommandHarness())
	}
	if os.Getenv(fakeHarnessEnvironment) == "1" {
		os.Exit(runFakeHarness())
	}
	os.Exit(m.Run())
}

func commandAcceptanceCollar(command string) string {
	return fmt.Sprintf(`package collars
collar: {
 name: "acceptance"
 launch: {command: %q, args: []}
 hooks: {
  install_args: ["--hook", "{{hook.command.json}}"]
  events: {
	   userpromptsubmit: {event: "turn-started", payload: {prompt: "prompt"}}
   stop: {event: "turn-finished"}
   precompact: {event: "compaction-started"}
   postcompact: {event: "compaction-finished"}
  }
 }
 models: {catalog: {name: "codex-debug-models", params: {command: "false", args: ""}}, option: {args: ["-m", "{{value}}"]}}
 options: {effort: {args: ["-e", "{{value}}"]}, role_prompt: {args: ["--role-prompt", "{{value}}"]}}
 primitives: {
	  startup: [{name: "claude-composer"}]
	  composer: {name: "claude-composer"}
	  submit: {name: "enter-submit"}
	  submit_witness: {name: "claude-pasted-content"}
	  turn_boundary: {name: "hook-boundary"}
	  blocked: {name: "screen-blocked", params: {prompt: "BLOCKED", choice: "ALLOW"}}
  context: {name: "codex-screen-context"}
  provider_limits: {name: "codex-screen-limits"}
  wedge: {name: "stable-busy-screen", params: {busy: "WORKING", after: "1ns"}}
 }
 actions: {interrupt: {keys: ["Escape"]}, compact: {text: "/compact {{instructions}}", submit: true}, compact_recover: [{keys: ["Escape"]}]}
 context_bands: {"*": [{name: "yellow", at: 0.75}]}
}
`, command)
}

func assertWindowName(t *testing.T, runner tmuxRunner, session, want string) {
	t.Helper()
	output, err := runner.run("list-windows", "-t", session, "-F", "#{window_name}")
	if err != nil {
		t.Fatalf("read window name: %v\n%s", err, output)
	}
	if got := strings.TrimSpace(output); got != want {
		t.Fatalf("window name = %q, want %q", got, want)
	}
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
	base := os.TempDir()
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
	if ready := os.Getenv("GANGLINE_ACCEPTANCE_READY"); ready != "" {
		if err := signal(ready); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
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

func runCommandHarness() int {
	argv := strings.Join(os.Args[1:], "\n") + "\n"
	if err := os.WriteFile(os.Getenv("GANGLINE_ACCEPTANCE_ARGV_LEDGER"), []byte(argv), 0o600); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	stty := exec.Command("stty", "raw", "-echo")
	stty.Stdin = os.Stdin
	if output, err := stty.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "set raw terminal: %v: %s", err, output)
		return 1
	}
	if started := os.Getenv("GANGLINE_ACCEPTANCE_STARTED_FIFO"); started != "" {
		if err := os.WriteFile(started, []byte("x"), 0o600); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	}
	if release := os.Getenv("GANGLINE_ACCEPTANCE_RELEASE_FIFO"); release != "" {
		if _, err := os.ReadFile(release); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	}
	renderCommandComposer("")
	if ready := os.Getenv("GANGLINE_ACCEPTANCE_READY"); ready != "" {
		if err := signal(ready); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	}
	reader := bufio.NewReader(os.Stdin)
	var input strings.Builder
	receivedCount := 0
	for {
		value, err := reader.ReadByte()
		if err != nil {
			if err == io.EOF {
				return 0
			}
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		if value == 2 {
			fmt.Print("\x1b[HWORKING")
			if err := signal("native-busy"); err != nil {
				fmt.Fprintln(os.Stderr, err)
				return 1
			}
			continue
		}
		if value != '\r' {
			input.WriteByte(value)
			if strings.HasSuffix(input.String(), "]") && strings.Contains(input.String(), "[/gang:") {
				renderCommandComposer(input.String())
			}
			continue
		}
		file, err := os.OpenFile(os.Getenv("GANGLINE_ACCEPTANCE_LEDGER"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		_, writeErr := fmt.Fprintln(file, input.String())
		closeErr := file.Close()
		if writeErr != nil || closeErr != nil {
			fmt.Fprintln(os.Stderr, writeErr, closeErr)
			return 1
		}
		if err := commandHarnessHook(input.String()); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		input.Reset()
		renderCommandComposer("")
		receivedCount++
		if err := signal(fmt.Sprintf("received-%s-%d", os.Getenv("GANGLINE_HITCH_ID"), receivedCount)); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	}
}

func commandHarnessHook(prompt string) error {
	commandText := ""
	for index := 1; index+1 < len(os.Args); index++ {
		if os.Args[index] == "--hook" {
			if err := json.Unmarshal([]byte(os.Args[index+1]), &commandText); err != nil {
				return fmt.Errorf("decode hook command: %w", err)
			}
			break
		}
	}
	if commandText == "" {
		return fmt.Errorf("fake harness received no hook command")
	}
	prompt = "\n\n<pasted_content id=\"acceptance\">\n" + prompt + "\n</pasted_content id=\"acceptance\">\n"
	payload, _ := json.Marshal(map[string]string{"hook_event_name": "UserPromptSubmit", "prompt": prompt})
	command := exec.Command("sh", "-c", commandText)
	command.Stdin = strings.NewReader(string(payload))
	if output, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("run hook: %w: %s", err, output)
	}
	return nil
}

func renderCommandComposer(input string) {
	const rule = "────────────────────────────────────────────────────────────"
	if len(input) > 120 {
		input = "[Pasted text]"
	}
	lines := strings.Split(input, "\n")
	if len(lines) == 0 {
		lines = []string{""}
	}
	fmt.Print("\x1b[2J\x1b[HREADY\r\n", rule, "\r\n❯ ", lines[0], "\r\n")
	for _, line := range lines[1:] {
		fmt.Print(line, "\r\n")
	}
	fmt.Print(rule, "\r\n")
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
