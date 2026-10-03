package acceptance

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/adambiggs/gangline/substrate/tmux"
)

const fakeHarnessEnvironment = "GANGLINE_ACCEPTANCE_FAKE_HARNESS"

func TestMain(m *testing.M) {
	// Fixture scripts run this binary to wait on a tmux channel with a bound;
	// they inherit the harness settings below, so it is checked first.
	if channel := os.Getenv("GANGLINE_ACCEPTANCE_BOUNDED_WAIT"); channel != "" {
		os.Exit(runBoundedWait(os.Args[1], channel))
	}
	if os.Getenv("GANGLINE_ACCEPTANCE_RESUME_HARNESS") == "1" {
		os.Exit(runResumeHarness())
	}
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
	return runner.runContext(context.Background(), arguments...)
}

// runContext runs a tmux command that ctx bounds, so a server that stops
// answering fails the test instead of holding it. A client passes its output
// descriptors to the server, so a stopped server holds the output pipe open
// after the client is killed; WaitDelay ends the read.
func (runner tmuxRunner) runContext(ctx context.Context, arguments ...string) (string, error) {
	arguments = append([]string{"-S", runner.socket, "-f", "/dev/null"}, arguments...)
	command := exec.CommandContext(ctx, runner.binary, arguments...)
	command.Env = runner.env
	command.WaitDelay = time.Second
	output, err := command.CombinedOutput()
	return string(output), err
}

// serverPID is the pid of the test's own tmux server, read while it answers.
func (runner tmuxRunner) serverPID(t *testing.T) int {
	t.Helper()
	out, err := runner.run("display-message", "-p", "#{pid}")
	if err != nil {
		t.Fatalf("read tmux server pid: %v %s", err, out)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(out))
	if err != nil {
		t.Fatalf("read tmux server pid: %v", err)
	}
	return pid
}

// nativeExitProbe bounds each tmux and ps call of a failure's diagnosis.
const nativeExitProbe = 5 * time.Second

// awaitNativeExit releases a native CLI waiting on channel, then waits for it
// to exit and for tmux to reap it. The pipe's EOF is the exit barrier: the
// native CLI holds its write end until it exits. The wait ends within a
// minute, and early enough before the test deadline for the diagnosis and the
// cleanup to finish, since the timeout panic discards the failure.
func awaitNativeExit(t *testing.T, runner tmuxRunner, server int, channel, pipe, trace string) {
	t.Helper()
	bound := time.Minute
	if deadline, ok := t.Deadline(); ok {
		bound = min(bound, time.Until(deadline)-4*nativeExitProbe)
	}
	ctx, cancel := context.WithTimeout(context.Background(), bound)
	defer cancel()
	exited := make(chan error, 1)
	go func() {
		reader, err := os.Open(pipe)
		if err != nil {
			exited <- err
			return
		}
		defer reader.Close()
		_, err = io.Copy(io.Discard, reader)
		exited <- err
	}()
	fail := func(format string, arguments ...any) {
		t.Helper()
		t.Fatalf(format+"\n%s", append(arguments, nativeExitDiagnosis(runner, server, trace))...)
	}
	if out, err := runner.runContext(ctx, "wait-for", "-S", channel); err != nil {
		fail("release native exit: %v %s", err, out)
	}
	select {
	case err := <-exited:
		if err != nil {
			fail("await native exit: %v", err)
		}
	case <-ctx.Done():
		fail("native CLI did not exit")
	}
	if out, err := tmux.Reaped(ctx, runner.runContext); err != nil {
		fail("reap native exit: %v %s", err, out)
	}
}

// nativeExitDiagnosis reports the native CLI's own record of its exit path,
// which needs no tmux to read, and what the tmux server shows if it answers.
// A server that does not answer is killed: the cleanup's kill-session would
// otherwise hold the test until the package timeout, which discards the
// failure this reports.
func nativeExitDiagnosis(runner tmuxRunner, server int, trace string) string {
	var report strings.Builder
	if record, err := os.ReadFile(trace); err != nil {
		fmt.Fprintf(&report, "native trace: %v\n", err)
	} else {
		fmt.Fprintf(&report, "native trace:\n%s", record)
	}
	ctx, cancel := context.WithTimeout(context.Background(), nativeExitProbe)
	defer cancel()
	panes, err := runner.runContext(ctx, "list-panes", "-a", "-F", "#{pane_id} #{window_name} pid=#{pane_pid} dead=#{pane_dead} status=#{pane_dead_status}")
	if ctx.Err() != nil {
		fmt.Fprintf(&report, "tmux server %d did not answer\n", server)
		// The pid is killed only while it still runs this test's server.
		check, cancel := context.WithTimeout(context.Background(), nativeExitProbe)
		defer cancel()
		command, err := exec.CommandContext(check, "ps", "-o", "command=", "-p", strconv.Itoa(server)).Output()
		if err == nil && strings.Contains(string(command), runner.socket) {
			fmt.Fprintf(&report, "killed it: %v\n", syscall.Kill(server, syscall.SIGKILL))
		}
		return report.String()
	}
	if err != nil {
		fmt.Fprintf(&report, "list panes: %v %s\n", err, panes)
		return report.String()
	}
	pids := []string{strconv.Itoa(server)}
	for _, line := range strings.Split(strings.TrimSpace(panes), "\n") {
		for _, field := range strings.Fields(line) {
			if pid, ok := strings.CutPrefix(field, "pid="); ok {
				pids = append(pids, pid)
			}
		}
	}
	processes, err := exec.CommandContext(ctx, "ps", "-o", "pid,ppid,stat,command", "-p", strings.Join(pids, ",")).CombinedOutput()
	fmt.Fprintf(&report, "panes:\n%s\nprocesses (%v):\n", panes, err)
	for _, line := range strings.Split(strings.TrimSpace(string(processes)), "\n") {
		fmt.Fprintf(&report, "%.160s\n", line)
	}
	return report.String()
}

// runBoundedWait waits on a channel of the tmux server at socket for at most a
// minute, so a signal that never comes fails the test instead of holding it.
func runBoundedWait(socket, channel string) int {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if output, err := exec.CommandContext(ctx, "tmux", "-S", socket, "wait-for", channel).CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "wait for %s: %v: %s", channel, err, output)
		return 1
	}
	return 0
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

// traceNative appends a step of the native CLI's exit path to the trace file a
// test names, so a test can tell where an exit stopped without its pane.
func traceNative(format string, arguments ...any) {
	path := os.Getenv("GANGLINE_ACCEPTANCE_NATIVE_TRACE")
	if path == "" {
		return
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o600)
	if err != nil {
		return
	}
	defer file.Close()
	fmt.Fprintf(file, "%d %s: "+format+"\n", append([]any{os.Getpid(), time.Now().Format(time.RFC3339Nano)}, arguments...)...)
}

func runCommandHarness() int {
	if failure := os.Getenv("GANGLINE_ACCEPTANCE_BOOT_EXIT"); failure != "" {
		if os.Getenv("GANGLINE_ACCEPTANCE_BOOT_BLOCKED") != "" {
			// A startup prompt the operator answers by ending the native CLI.
			fmt.Println("BLOCKED ALLOW")
		}
		if channel := os.Getenv("GANGLINE_ACCEPTANCE_BOOT_EXIT_AFTER"); channel != "" {
			traceNative("await %s", channel)
			// Bounded so a hitch that never signals cannot hold the pane forever.
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			if output, err := exec.CommandContext(ctx, "tmux", "-S", os.Getenv("GANG_TMUX_SOCKET"), "wait-for", channel).CombinedOutput(); err != nil {
				traceNative("await %s failed: %v: %s", channel, err, output)
				fmt.Fprintf(os.Stderr, "await boot exit: %v: %s", err, output)
				return 1
			}
			traceNative("released by %s", channel)
		}
		// The write end stays open until the process exits, so the reader's EOF
		// is an exit barrier. A raw descriptor has no finalizer to close it early.
		if pipe := os.Getenv("GANGLINE_ACCEPTANCE_EXIT_PIPE"); pipe != "" {
			if _, err := syscall.Open(pipe, syscall.O_WRONLY, 0); err != nil {
				traceNative("open exit pipe: %v", err)
				fmt.Fprintln(os.Stderr, err)
				return 1
			}
			traceNative("exit pipe open")
		}
		traceNative("exit 7")
		fmt.Fprintln(os.Stderr, failure)
		return 7
	}
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
		if failure := os.Getenv("GANGLINE_ACCEPTANCE_INPUT_EXIT"); failure != "" {
			// A native CLI that ends on the first input it is given.
			fmt.Fprint(os.Stderr, "\r\n", failure, "\r\n")
			return 7
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
		// A gang command typed into the pane runs with the pane's environment,
		// as a native CLI's shell tool would.
		arguments, ok := strings.CutPrefix(input.String(), "__GANG__ ")
		ending, ends := strings.CutPrefix(input.String(), "__GANG_ENDS_PANE__ ")
		if ends {
			arguments = ending
		}
		if ok || ends {
			command := exec.Command(os.Getenv("GANGLINE_ACCEPTANCE_GANG"), strings.Fields(arguments)...)
			if ends {
				// A command that can end this pane hands its exit to a pipe
				// only it and its children hold, so the test sees the exit
				// with the pane gone.
				// Its output goes to a pipe this process reads, as a native
				// CLI reads a command's output, and loses its reader with the pane.
				exit, err := os.OpenFile(os.Getenv("GANGLINE_ACCEPTANCE_PANE_EXIT"), os.O_WRONLY, 0)
				if err != nil {
					fmt.Fprintln(os.Stderr, err)
					return 1
				}
				var output bytes.Buffer
				command.Stdout, command.Stderr = &output, &output
				command.ExtraFiles = []*os.File{exit}
				err = command.Start()
				if closeErr := exit.Close(); err != nil || closeErr != nil {
					fmt.Fprintln(os.Stderr, err, closeErr)
					return 1
				}
				_ = command.Wait()
				fmt.Print(output.String())
				input.Reset()
				renderCommandComposer("")
				continue
			}
			output, err := command.CombinedOutput()
			status := 0
			if err != nil {
				var exit *exec.ExitError
				if errors.As(err, &exit) {
					status = exit.ExitCode()
				} else {
					status = -1
				}
			}
			if err := os.WriteFile(os.Getenv("GANGLINE_ACCEPTANCE_PANE_RESULT"), []byte(fmt.Sprintf("status=%d\n%s", status, output)), 0o600); err != nil {
				fmt.Fprintln(os.Stderr, err)
				return 1
			}
			ready, err := os.OpenFile(os.Getenv("GANGLINE_ACCEPTANCE_PANE_READY"), os.O_WRONLY, 0)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				return 1
			}
			_, writeErr := ready.Write([]byte("x"))
			closeErr := ready.Close()
			if writeErr != nil || closeErr != nil {
				fmt.Fprintln(os.Stderr, writeErr, closeErr)
				return 1
			}
			input.Reset()
			renderCommandComposer("")
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
