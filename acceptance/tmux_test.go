package acceptance

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/store"
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

func TestCommandLifecycleOnPrivateTmux(t *testing.T) {
	tmuxBinary, err := exec.LookPath("tmux")
	if err != nil {
		t.Fatal("tmux is required for acceptance tests")
	}
	goBinary, err := exec.LookPath("go")
	if err != nil {
		t.Fatal("go is required for command acceptance tests")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	base := filepath.Join(home, ".local", "state", "gangline", "acceptance")
	if err := os.MkdirAll(base, 0o700); err != nil {
		t.Fatal(err)
	}
	root, err := os.MkdirTemp(base, "command-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })

	repository, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	gangBinary := filepath.Join(root, "gang")
	build := exec.Command(goBinary, "build", "-o", gangBinary, "./cmd/gang")
	build.Dir = repository
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build gang: %v\n%s", err, output)
	}
	testBinary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	collarDir := filepath.Join(root, "collars")
	if err := os.Mkdir(collarDir, 0o700); err != nil {
		t.Fatal(err)
	}
	configDir := filepath.Join(root, "config")
	if err := os.Mkdir(configDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(configDir, "roles"), 0o700); err != nil {
		t.Fatal(err)
	}
	const leadBrief = "ACCEPTANCE LEAD ROLE BRIEF"
	if err := os.WriteFile(filepath.Join(configDir, "roles", "lead.md"), []byte(leadBrief+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	collar := commandAcceptanceCollar(testBinary)
	if err := os.WriteFile(filepath.Join(collarDir, "acceptance.cue"), []byte(collar), 0o600); err != nil {
		t.Fatal(err)
	}

	socket := filepath.Join(root, "tmux.sock")
	ledger := filepath.Join(root, "received.log")
	argvLedger := filepath.Join(root, "argv.log")
	startedFIFO := filepath.Join(root, "harness-started.fifo")
	releaseFIFO := filepath.Join(root, "harness-release.fifo")
	for _, fifo := range []string{startedFIFO, releaseFIFO} {
		if err := syscall.Mkfifo(fifo, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	const session = "gangline-command-acceptance"
	environment := append(withoutEnvironment(os.Environ(), "TMUX", "TMUX_PANE", "GANG_CONFIG_DIR"),
		"GANG_SESSION="+session,
		"GANG_CONFIG_DIR="+configDir,
		"GANG_STATE_ROOT="+filepath.Join(root, "state"),
		"GANG_TMUX_SOCKET="+socket,
		"GANG_COLLARS="+collarDir,
		"GANG_COLLAR=acceptance",
		`GANG_LAUNCH_ARGS={"acceptance":["--operator-unsandboxed"]}`,
		"GANGLINE_ACCEPTANCE_CMD_HARNESS=1",
		"GANGLINE_ACCEPTANCE_TMUX="+tmuxBinary,
		"GANGLINE_ACCEPTANCE_TMUX_SOCKET="+socket,
		"GANGLINE_ACCEPTANCE_STARTED_FIFO="+startedFIFO,
		"GANGLINE_ACCEPTANCE_RELEASE_FIFO="+releaseFIFO,
		"GANGLINE_ACCEPTANCE_LEDGER="+ledger,
		"GANGLINE_ACCEPTANCE_ARGV_LEDGER="+argvLedger,
	)
	runner := tmuxRunner{binary: tmuxBinary, socket: socket, env: environment}
	const proofSession = "gangline-window-mark-proof"
	if output, err := runner.run("new-session", "-d", "-s", proofSession); err != nil {
		t.Fatalf("create private proof session: %v\n%s", err, output)
	}
	if output, err := runner.run("set-hook", "-g", "after-rename-window", "wait-for -S command-harness-marked"); err != nil {
		t.Fatalf("install window-mark barrier: %v\n%s", err, output)
	}
	t.Cleanup(func() {
		_, _ = runner.run("kill-session", "-t", session)
		_, _ = runner.run("kill-session", "-t", proofSession)
	})
	runGang := func(input string, arguments ...string) (string, int) {
		t.Helper()
		command := exec.Command(gangBinary, arguments...)
		command.Dir = repository
		command.Env = environment
		command.Stdin = strings.NewReader(input)
		output, err := command.CombinedOutput()
		if err == nil {
			return string(output), 0
		}
		if exit, ok := err.(*exec.ExitError); ok {
			return string(output), exit.ExitCode()
		}
		t.Fatalf("run gang %v: %v", arguments, err)
		return "", -1
	}

	receivedCount := 0
	waitReceived := func() (string, error) {
		receivedCount++
		return runner.run("wait-for", fmt.Sprintf("received-%d", receivedCount))
	}
	up := exec.Command(gangBinary, "up", "-c", "acceptance", "--stdin")
	up.Dir = repository
	up.Env = environment
	up.Stdin = strings.NewReader("acceptance assignment\n")
	var upOutput bytes.Buffer
	up.Stdout, up.Stderr = &upOutput, &upOutput
	if err := up.Start(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.ReadFile(startedFIFO); err != nil {
		t.Fatalf("wait for harness process: %v", err)
	}
	if output, err := runner.run("wait-for", "command-harness-marked"); err != nil {
		t.Fatalf("wait for booting window mark: %v\n%s", err, output)
	}
	if output, err := runner.run("set-hook", "-gu", "after-rename-window"); err != nil {
		t.Fatalf("remove window-mark barrier: %v\n%s", err, output)
	}
	assertWindowName(t, runner, session, "?lead?")
	if err := os.WriteFile(releaseFIFO, []byte("x"), 0o600); err != nil {
		t.Fatalf("release harness composer: %v", err)
	}
	if err := up.Wait(); err != nil {
		t.Fatalf("up did not observe the released composer: %v\n%s", err, upOutput.String())
	}
	startupState := loadAcceptanceState(t, filepath.Join(root, "state"), session)
	startupDelivered := false
	for _, delivery := range startupState.Deliveries {
		startupDelivered = startupDelivered || delivery.Status == core.DeliveryDelivered
		if delivery.Status != core.DeliveryDelivered {
			t.Fatalf("startup delivery status = %q: %s", delivery.Status, delivery.Reason)
		}
	}
	if !startupDelivered {
		t.Fatal("startup delivery was not recorded")
	}
	if output, status := runGang("", "capture", "lead"); status != 0 || !strings.Contains(output, "READY") {
		t.Fatalf("capture decorated agent status %d:\n%s", status, output)
	}
	if output, err := waitReceived(); err != nil {
		t.Fatalf("wait for startup delivery: %v\n%s", err, output)
	}
	assertWindowName(t, runner, session, "-lead-")
	argv, err := os.ReadFile(argvLedger)
	if err != nil || !strings.Contains(string(argv), "--operator-unsandboxed") || !strings.Contains(string(argv), leadBrief) {
		t.Fatalf("operator launch policy was not observed: %v %q", err, argv)
	}
	state := loadAcceptanceState(t, filepath.Join(root, "state"), session)
	hitch, ok := acceptanceHitch(state, "lead")
	if !ok {
		t.Fatal("lead did not become active")
	}
	if hitch.Role != "lead" {
		t.Fatalf("lead role = %q, want lead", hitch.Role)
	}
	if hitch.Directory != repository {
		t.Fatalf("lead directory = %q, want caller cwd %q", hitch.Directory, repository)
	}
	whoami := exec.Command(gangBinary, "whoami")
	whoami.Dir = repository
	whoami.Env = append(environment, "TMUX_PANE="+hitch.Pane)
	if output, err := whoami.CombinedOutput(); err != nil || strings.TrimSpace(string(output)) != "lead" {
		t.Fatalf("whoami: %v, output %q", err, output)
	}

	hookEnv := append(environment, "GANGLINE_HITCH_ID="+string(hitch.ID))
	runHook := func(native string) {
		t.Helper()
		payload, _ := json.Marshal(map[string]string{"hook_event_name": native})
		command := exec.Command(gangBinary, "hook")
		command.Dir, command.Env, command.Stdin = repository, hookEnv, strings.NewReader(string(payload))
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("hook %s: %v\n%s", native, err, output)
		}
	}
	runHook("Stop")
	assertWindowName(t, runner, session, "~lead~")
	if output, status := runGang("ordinary delivery\n", "send", "lead", "--from", "tester"); status != 0 {
		screen, _ := runner.run("capture-pane", "-p", "-J", "-t", session)
		t.Fatalf("send status %d:\n%s\nscreen:\n%s", status, output, screen)
	}
	if output, err := waitReceived(); err != nil {
		t.Fatalf("wait for ordinary delivery: %v\n%s", err, output)
	}
	runHook("Stop")
	const multiline = "multiline first\nmultiline second\n"
	if output, status := runGang(multiline, "send", "lead", "--from", "tester"); status != 0 {
		t.Fatalf("normalized multiline send status %d:\n%s", status, output)
	}
	if output, err := waitReceived(); err != nil {
		t.Fatalf("wait for normalized multiline delivery: %v\n%s", err, output)
	}
	state = loadAcceptanceState(t, filepath.Join(root, "state"), session)
	if !hasAcceptanceDelivery(state, strings.TrimSuffix(multiline, "\n"), core.DeliveryDelivered) {
		t.Fatalf("normalized multiline prompt was not verified: %#v", state.Deliveries)
	}
	t.Run("tick retries a durable queued send", func(t *testing.T) {
		output, status := runGang("queued delivery\n", "send", "lead", "--from", "tester")
		if status != 0 || !strings.Contains(output, "\tqueued\n") {
			t.Fatalf("queue status %d, want queued:\n%s", status, output)
		}
		queued := loadAcceptanceState(t, filepath.Join(root, "state"), session)
		if !hasAcceptanceDelivery(queued, "queued delivery", core.DeliveryQueued) {
			t.Fatalf("send did not remain durably queued: %#v", queued.Deliveries)
		}
		if output, status := runGang("", "tick"); status != 0 {
			t.Fatalf("queued retry status %d:\n%s", status, output)
		}
		if output, err := waitReceived(); err != nil {
			t.Fatalf("wait for queued retry: %v\n%s", err, output)
		}
		delivered := loadAcceptanceState(t, filepath.Join(root, "state"), session)
		if !hasAcceptanceDelivery(delivered, "queued delivery", core.DeliveryDelivered) {
			t.Fatalf("tick did not deliver queued send: %#v", delivered.Deliveries)
		}
	})
	runHook("Stop")
	if output, status := runGang("", "compact", "lead", "--resume", "resume after compact"); status != 0 {
		t.Fatalf("compact status %d:\n%s", status, output)
	}
	if output, err := waitReceived(); err != nil {
		t.Fatalf("wait for compact command: %v\n%s", err, output)
	}
	runHook("PostCompact")
	if output, err := waitReceived(); err != nil {
		t.Fatalf("wait for compaction continuation: %v\n%s", err, output)
	}
	runHook("Stop")
	runHook("UserPromptSubmit")
	if output, err := runner.run("send-keys", "-t", string(hitch.Pane), "C-b"); err != nil {
		t.Fatalf("set native busy: %v\n%s", err, output)
	}
	if output, err := runner.run("wait-for", "native-busy"); err != nil {
		t.Fatalf("observe native busy: %v\n%s", err, output)
	}
	if output, status := runGang("", "tick"); status != 0 {
		t.Fatalf("first wedge observation status %d:\n%s", status, output)
	}
	if output, status := runGang("", "tick"); status != 0 {
		t.Fatalf("second wedge observation status %d:\n%s", status, output)
	}
	state = loadAcceptanceState(t, filepath.Join(root, "state"), session)
	hitch, _ = acceptanceHitch(state, "lead")
	if hitch.Activity != core.ActivityWedged {
		t.Fatalf("activity = %q, want wedged", hitch.Activity)
	}
	assertWindowName(t, runner, session, "!lead!")
	if output, status := runGang("", "drop", "lead"); status != 0 {
		t.Fatalf("drop status %d:\n%s", status, output)
	}
	state = loadAcceptanceState(t, filepath.Join(root, "state"), session)
	for _, recorded := range state.Hitches {
		if recorded.Name == "lead" && recorded.Status != core.HitchDropped {
			t.Fatalf("drop left status %q", recorded.Status)
		}
	}
	received, err := os.ReadFile(ledger)
	if err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"assignment", "ordinary delivery", "multiline first", "multiline second", "queued delivery", "/compact resume after compact", "resume after compact"} {
		if !strings.Contains(string(received), text) {
			t.Fatalf("delivery ledger lacks %q:\n%s", text, received)
		}
	}
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

func loadAcceptanceState(t *testing.T, root, session string) core.State {
	t.Helper()
	// Native hook diagnostics may still be appending after delivery is witnessed.
	// Acquire the transaction barrier before inspecting the committed state.
	locked, err := (store.Paths{Root: root}).LockWait(session)
	if err != nil {
		t.Fatal(err)
	}
	defer locked.Close()
	state, _, err := locked.Load(core.NewState(core.Team{ID: core.TeamID(session), Name: session}))
	if err != nil {
		t.Fatal(err)
	}
	return state
}

func acceptanceHitch(state core.State, name string) (core.Hitch, bool) {
	for _, hitch := range state.Hitches {
		if hitch.Name == core.AgentName(name) {
			return hitch, true
		}
	}
	return core.Hitch{}, false
}

func hasAcceptanceDelivery(state core.State, text string, status core.DeliveryStatus) bool {
	for _, delivery := range state.Deliveries {
		if delivery.Envelope.Message.Text == text && delivery.Status == status {
			return true
		}
	}
	return false
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
			renderCommandComposer(input.String())
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
		if err := signal(fmt.Sprintf("received-%d", receivedCount)); err != nil {
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
