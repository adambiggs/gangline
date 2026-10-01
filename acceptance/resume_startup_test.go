package acceptance

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/store"
)

// FIFO events distinguish a native submission from input consumed by copy mode.
// Neither outcome needs a polling loop or a wall-clock deadline.
func TestResumedLeadCopyModeStartupRecoveryAndSends(t *testing.T) {
	root, err := os.MkdirTemp("", "resume-")
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
		t.Fatalf("build gang: %v: %s", err, out)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	collars := filepath.Join(root, "collars")
	if err := os.Mkdir(collars, 0700); err != nil {
		t.Fatal(err)
	}
	collar := strings.Replace(commandAcceptanceCollar(executable), "args: []", `args: [], resume_args: ["--resume", "{{session_id}}"]`, 1)
	collar = strings.Replace(collar, `payload: {prompt: "prompt"}`, `payload: {prompt: "prompt", session_id: "session_id"}`, 1)
	if err := os.WriteFile(filepath.Join(collars, "acceptance.cue"), []byte(collar), 0600); err != nil {
		t.Fatal(err)
	}
	eventPath := filepath.Join(root, "events")
	if out, err := exec.Command("mkfifo", eventPath).CombinedOutput(); err != nil {
		t.Fatalf("create event pipe: %v: %s", err, out)
	}
	events, err := os.OpenFile(eventPath, os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = events.Close() })
	reader := bufio.NewReader(events)
	const session = "resume-startup-acceptance"
	const nativeSession = "acceptance-resumed-session"
	socket := filepath.Join(root, "tmux.sock")
	env := append(withoutEnvironment(os.Environ(), "TMUX", "TMUX_PANE", "GANG_CONFIG_DIR", "GANG_SESSION", "GANG_STATE_ROOT", "GANG_TMUX_SOCKET", "GANG_COLLARS", "GANG_COLLAR", "GANGLINE_HITCH_ID", "GANG_AGENT_ID", "GANG_AGENT_NONCE", "GANG_AGENT_TOKEN"),
		"GANG_SESSION="+session, "GANG_CONFIG_DIR="+filepath.Join(root, "config"), "GANG_STATE_ROOT="+filepath.Join(root, "state"), "GANG_TMUX_SOCKET="+socket, "GANG_COLLARS="+collars, "GANG_COLLAR=acceptance",
		"GANGLINE_ACCEPTANCE_RESUME_HARNESS=1", "GANGLINE_ACCEPTANCE_ROOT="+root, "GANGLINE_ACCEPTANCE_GANG="+binary)
	runner := tmuxRunner{binary: "tmux", socket: socket, env: env}
	mustResumeTmux(t, runner, "new-session", "-d", "-s", session, "-n", "control", "-x", "240", "-y", "60")
	t.Cleanup(func() { _, _ = runner.run("kill-session", "-t", session) })
	if got := strings.TrimSpace(mustResumeTmux(t, runner, "list-sessions", "-F", "#{session_name}")); got != session {
		t.Fatalf("private server sessions = %q, want %q", got, session)
	}
	// Keep copy mode active while literal input is mistakenly interpreted as keys.
	for _, table := range []string{"copy-mode", "copy-mode-vi"} {
		mustResumeTmux(t, runner, "unbind-key", "-a", "-T", table)
		mustResumeTmux(t, runner, "bind-key", "-T", table, "[", "run-shell", "printf 'copy-mode-input\\n' > "+shellQuote(eventPath))
		mustResumeTmux(t, runner, "bind-key", "-T", table, "Enter", "run-shell", "printf 'copy-mode-enter\\n' > "+shellQuote(eventPath))
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	runGang := func(args ...string) (string, error) {
		command := exec.CommandContext(ctx, binary, args...)
		command.Env, command.Dir, command.Stdin = env, repo, strings.NewReader("")
		out, err := command.CombinedOutput()
		return string(out), err
	}
	t.Cleanup(func() {
		if _, err := os.Stat(filepath.Join(root, "state", "teams", session)); err == nil {
			command := exec.Command(binary, "roster")
			command.Env = env
			_, _ = command.CombinedOutput()
			command = exec.Command(binary, "down", "--yes")
			command.Env = env
			if out, err := command.CombinedOutput(); err != nil {
				t.Errorf("remove disposable team: %v: %s", err, out)
			}
		}
	})
	expectEvent := func(t *testing.T, want string) {
		t.Helper()
		got, err := reader.ReadString('\n')
		if err != nil || strings.TrimSpace(got) != want {
			cancel()
			t.Fatalf("native event = %q (%v), want %q", got, err, want)
		}
	}
	startGang := func(args ...string) <-chan error {
		result := make(chan error, 1)
		go func() {
			out, err := runGang(args...)
			if err != nil {
				failure := fmt.Errorf("gang %v: %w: %s", args, err, out)
				_, _ = fmt.Fprintln(events, failure)
				result <- failure
				return
			}
			result <- nil
		}()
		return result
	}
	var up <-chan error
	var operatorInput *os.File
	attached := false
	if script, err := exec.LookPath("script"); runtime.GOOS == "linux" && err == nil {
		mustResumeTmux(t, runner, "set-hook", "-t", session, "client-attached", "run-shell "+shellQuote("printf 'attached\\n' > "+shellQuote(eventPath)))
		input, writer, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		operatorInput = writer
		t.Cleanup(func() { _ = input.Close(); _ = writer.Close() })
		command := exec.CommandContext(ctx, script, "-qefc", "stty rows 60 cols 240; exec "+shellQuote(binary)+" up --resume "+shellQuote(nativeSession), "/dev/null")
		command.Env = append(withoutEnvironment(env, "TERM"), "TERM=xterm-256color")
		command.Dir, command.Stdin = repo, input
		result := make(chan error, 1)
		up = result
		go func() {
			out, err := command.CombinedOutput()
			if err != nil {
				failure := fmt.Errorf("terminal gang up: %w: %s", err, out)
				_, _ = fmt.Fprintln(events, failure)
				result <- failure
				return
			}
			result <- nil
		}()
		attached = true
	} else {
		t.Log("terminal attachment unproven: requires Linux util-linux script; exercising nonterminal startup")
		up = startGang("up", "--resume", nativeSession)
	}
	if attached {
		remaining := map[string]bool{"received": true, "attached": true}
		for range 2 {
			got, err := reader.ReadString('\n')
			event := strings.TrimSpace(got)
			if err != nil || !remaining[event] {
				cancel()
				t.Fatalf("startup event = %q (%v), remaining %v", got, err, remaining)
			}
			delete(remaining, event)
		}
	} else {
		expectEvent(t, "received")
		if err := <-up; err != nil {
			t.Fatal(err)
		}
	}
	team, err := (store.Paths{Root: filepath.Join(root, "state")}).Team(session)
	if err != nil {
		t.Fatal(err)
	}
	readAgent := func(t *testing.T, name string) (store.AgentPaths, core.Agent) {
		t.Helper()
		id, err := team.ResolveName(name)
		if err != nil {
			t.Fatal(err)
		}
		paths, err := team.Agent(id)
		if err != nil {
			t.Fatal(err)
		}
		agent, err := paths.Read()
		if err != nil {
			t.Fatal(err)
		}
		return paths, agent
	}
	leadPaths, lead := readAgent(t, "lead")
	if lead.Status != core.Active || lead.Native.SessionID != nativeSession || lead.LastDelivered == "" {
		t.Fatalf("resumed lead is not verified: %+v", lead)
	}
	argv, err := os.ReadFile(filepath.Join(root, string(lead.ID)+".argv"))
	if err != nil || !strings.HasPrefix(string(argv), "--resume\n"+nativeSession+"\n") {
		t.Fatalf("native resume argv = %q: %v", argv, err)
	}
	if mode := strings.TrimSpace(mustResumeTmux(t, runner, "display-message", "-p", "-t", lead.Pane, "#{pane_in_mode}")); mode != "0" {
		t.Fatalf("verified lead remains in tmux mode: %s", mode)
	}
	startup, err := leadPaths.ReadEnvelope("cur", lead.LastDelivered)
	if err != nil || startup.Outcome != "delivered" {
		t.Fatalf("startup receipt = %+v: %v", startup, err)
	}
	if attached {
		clients := strings.TrimSpace(mustResumeTmux(t, runner, "list-clients", "-t", session, "-F", "#{client_tty}\t#{session_name}"))
		tty, clientSession, ok := strings.Cut(clients, "\t")
		if !ok || tty == "" || clientSession != session || strings.Contains(clients, "\n") {
			t.Fatalf("operator attachment is not the sole disposable client: %q", clients)
		}
		mustResumeTmux(t, runner, "detach-client", "-t", tty)
		if err := operatorInput.Close(); err != nil {
			t.Fatal(err)
		}
		if err := <-up; err != nil {
			t.Fatal(err)
		}
		t.Log("operator terminal attached after verified resumed startup; detached exact disposable client")
	}
	// Model an interrupted submission with the original exact draft retained.
	// This avoids waiting for a production timeout merely to construct recovery state.
	wire, err := os.ReadFile(filepath.Join(root, string(lead.ID)+".received"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "draft"), wire, 0600); err != nil {
		t.Fatal(err)
	}
	locked, err := leadPaths.LockAgent()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(leadPaths.Witness); err != nil {
		t.Fatal(err)
	}
	if err := locked.Settle(&lead, startup, "unverified", "submission interrupted with retained draft"); err != nil {
		t.Fatal(err)
	}
	if err := locked.Close(); err != nil {
		t.Fatal(err)
	}
	enter := func(t *testing.T, pane, text string) {
		t.Helper()
		mustResumeTmux(t, runner, "send-keys", "-t", pane, "-l", text)
		mustResumeTmux(t, runner, "send-keys", "-t", pane, "Enter")
	}
	enter(t, lead.Pane, "__DRAFT__")
	expectEvent(t, "draft-ready")
	barrier, err := leadPaths.LockAgent()
	if err != nil {
		t.Fatal(err)
	}
	if err := barrier.Close(); err != nil {
		t.Fatal(err)
	}
	recovery := startGang("hitch", "lead", "--recover")
	expectEvent(t, "received")
	if err := <-recovery; err != nil {
		t.Fatal(err)
	}
	_, lead = readAgent(t, "lead")
	if recovered, err := leadPaths.ReadEnvelope("cur", startup.ID); err != nil || recovered.Outcome != "delivered" || recovered.Token != startup.Token || recovered.Message != startup.Message {
		t.Fatalf("recovery changed retained startup: %+v: %v", recovered, err)
	}
	ownerLaunch := startGang("hitch", "owner")
	expectEvent(t, "received")
	if err := <-ownerLaunch; err != nil {
		t.Fatal(err)
	}
	_, owner := readAgent(t, "owner")
	for _, direction := range []struct {
		sender, recipient core.Agent
		body              string
	}{{owner, lead, "owner to resumed lead"}, {lead, owner, "resumed lead to owner"}} {
		enter(t, direction.sender.Pane, "__SEND__"+string(direction.recipient.Name)+":"+direction.body)
		expectEvent(t, "received")
		expectEvent(t, "send-complete")
		paths, recipient := readAgent(t, string(direction.recipient.Name))
		receipt, err := paths.ReadEnvelope("cur", recipient.LastDelivered)
		if err != nil || receipt.Outcome != "delivered" || receipt.From.Kind != core.SenderAgent || receipt.From.HitchID != direction.sender.ID || receipt.Message.Text != direction.body {
			t.Fatalf("agent send receipt = %+v: %v", receipt, err)
		}
	}
	t.Run("sandboxed owner", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("PID namespace acceptance requires Linux")
		}
		if _, err := exec.LookPath("bwrap"); err != nil {
			t.Skip("PID namespace acceptance requires bwrap")
		}
		enter(t, owner.Pane, "__SANDBOX_SEND__lead:sandboxed owner to resumed lead")
		expectEvent(t, "received")
		expectEvent(t, "send-complete")
		paths, recipient := readAgent(t, "lead")
		receipt, err := paths.ReadEnvelope("cur", recipient.LastDelivered)
		if err != nil || receipt.Outcome != "delivered" || receipt.From.Kind != core.SenderAgent || receipt.From.HitchID != owner.ID || receipt.Message.Text != "sandboxed owner to resumed lead" {
			t.Fatalf("sandboxed send receipt = %+v: %v", receipt, err)
		}
		ownerPaths, _ := readAgent(t, "owner")
		if _, err := os.Stat(filepath.Join(ownerPaths.Directory, "process-unavailable")); err != nil {
			t.Fatalf("sandboxed send did not exercise unavailable host process visibility: %v", err)
		}
	})
}

func runResumeHarness() int {
	if err := resumeHarness(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		_ = resumeHarnessEvent("fixture-error: " + err.Error())
		return 1
	}
	return 0
}

func resumeHarnessEvent(event string) error {
	return os.WriteFile(filepath.Join(os.Getenv("GANGLINE_ACCEPTANCE_ROOT"), "events"), []byte(event+"\n"), 0600)
}

func resumeHarness() error {
	root, id := os.Getenv("GANGLINE_ACCEPTANCE_ROOT"), os.Getenv("GANGLINE_HITCH_ID")
	if err := os.WriteFile(filepath.Join(root, id+".argv"), []byte(strings.Join(os.Args[1:], "\n")+"\n"), 0600); err != nil {
		return err
	}
	stty := exec.Command("stty", "raw", "-echo")
	stty.Stdin = os.Stdin
	if out, err := stty.CombinedOutput(); err != nil {
		return fmt.Errorf("set raw terminal: %w: %s", err, out)
	}
	runner := tmuxRunner{binary: "tmux", socket: os.Getenv("GANG_TMUX_SOCKET"), env: os.Environ()}
	copyMode := func() error {
		out, err := runner.run("copy-mode", "-t", os.Getenv("TMUX_PANE"))
		if err != nil {
			return fmt.Errorf("copy mode: %w: %s", err, out)
		}
		return nil
	}
	// Enter the mode before publishing a ready composer, eliminating a race with up.
	if err := copyMode(); err != nil {
		return err
	}
	renderResumeComposer("")
	reader := bufio.NewReader(os.Stdin)
	var input strings.Builder
	for {
		value, err := reader.ReadByte()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if value != '\r' {
			input.WriteByte(value)
			if strings.HasSuffix(input.String(), "]") && strings.Contains(input.String(), "[/gang:") {
				renderResumeComposer(input.String())
			}
			continue
		}
		text := input.String()
		input.Reset()
		if text == "__DRAFT__" {
			draft, err := os.ReadFile(filepath.Join(root, "draft"))
			if err != nil {
				return err
			}
			input.Write(draft)
			renderResumeComposer(string(draft))
			if err := copyMode(); err != nil {
				return err
			}
			if err := resumeHarnessEvent("draft-ready"); err != nil {
				return err
			}
			continue
		}
		command, send := strings.CutPrefix(text, "__SEND__")
		sandboxCommand, sandbox := strings.CutPrefix(text, "__SANDBOX_SEND__")
		if sandbox {
			command = sandboxCommand
		}
		if send || sandbox {
			recipient, body, _ := strings.Cut(command, ":")
			cmd := exec.Command(os.Getenv("GANGLINE_ACCEPTANCE_GANG"), "send", recipient, body)
			if sandbox {
				cmd = exec.Command("bwrap", "--unshare-user", "--unshare-pid", "--bind", "/", "/", "--proc", "/proc", "--dev", "/dev", "--", os.Getenv("GANGLINE_ACCEPTANCE_GANG"), "send", recipient, body)
			}
			if out, err := cmd.CombinedOutput(); err != nil || !strings.HasSuffix(strings.TrimSpace(string(out)), "\tdelivered") {
				return fmt.Errorf("native send: %v: %s", err, out)
			}
			renderResumeComposer("")
			if err := resumeHarnessEvent("send-complete"); err != nil {
				return err
			}
			continue
		}
		if err := os.WriteFile(filepath.Join(root, id+".received"), []byte(text), 0600); err != nil {
			return err
		}
		if err := resumeHarnessHook(text); err != nil {
			return err
		}
		renderResumeComposer("")
		if err := resumeHarnessEvent("received"); err != nil {
			return err
		}
	}
}

// Publish the fixture's exact submit witness synchronously. Real hook dispatch
// is covered by the command lifecycle acceptance test; its detached tick would
// race with the deliberately constructed interrupted-startup state here.
func resumeHarnessHook(prompt string) error {
	session := ""
	for index := 1; index+1 < len(os.Args); index++ {
		if os.Args[index] == "--resume" {
			session = os.Args[index+1]
		}
	}
	team, err := (store.Paths{Root: os.Getenv("GANG_STATE_ROOT")}).Team(os.Getenv("GANG_SESSION"))
	if err != nil {
		return err
	}
	paths, err := team.Agent(core.HitchID(os.Getenv("GANGLINE_HITCH_ID")))
	if err != nil {
		return err
	}
	now := time.Now()
	return paths.WriteWitness(store.Witness{ID: fmt.Sprintf("fixture-%d", now.UnixNano()), At: now, SessionID: session, Prompt: "\n\n<pasted_content id=\"acceptance\">\n" + prompt + "\n</pasted_content id=\"acceptance\">\n"})
}

func renderResumeComposer(input string) {
	const rule = "────────────────────────────────────────────────────────────"
	fmt.Print("\x1b[2J\x1b[HREADY\r\n", rule, "\r\n❯ ", strings.ReplaceAll(input, "\n", "\r\n"), "\r\n", rule, "\r\n")
}

func mustResumeTmux(t *testing.T, runner tmuxRunner, args ...string) string {
	t.Helper()
	output, err := runner.run(args...)
	if err != nil {
		t.Fatalf("private tmux %v: %v: %s", args, err, output)
	}
	return output
}
