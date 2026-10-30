//go:build acceptance

package acceptance

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	ossignal "os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/store"
)

func TestSelfRehitchOnPrivateTmux(t *testing.T) {
	root := t.TempDir()
	repo, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(root, "gang")
	build := exec.Command("go", "build", "-o", binary, "./cmd/gang")
	build.Dir = repo
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v: %s", err, out)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	collars := filepath.Join(root, "collars")
	if err := os.Mkdir(collars, 0700); err != nil {
		t.Fatal(err)
	}
	collar := strings.Replace(commandAcceptanceCollar(exe), "args: []", `args: [], resume_args: ["--resume", "{{session_id}}"], upgrade_args: ["--fixture-update"]`, 1)
	collar = strings.Replace(collar, `effort: {args: ["-e", "{{value}}"]}, `, "", 1)
	if strings.Contains(collar, "effort:") {
		t.Fatal("restart fixture declares an effort option")
	}
	collar = strings.Replace(collar, `payload: {prompt: "prompt"}`, `payload: {prompt: "prompt", session_id: "session_id"}`, 1)
	if err := os.WriteFile(filepath.Join(collars, "acceptance.cue"), []byte(collar), 0600); err != nil {
		t.Fatal(err)
	}
	fifo := filepath.Join(root, "events")
	if out, err := exec.Command("mkfifo", fifo).CombinedOutput(); err != nil {
		t.Fatalf("mkfifo: %v: %s", err, out)
	}
	events, err := os.OpenFile(fifo, os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer events.Close()
	reader := bufio.NewReader(events)
	const session = "self-rehitch-acceptance"
	socket := filepath.Join(root, "tmux.sock")
	env := append(withoutEnvironment(os.Environ(), "TMUX", "TMUX_PANE", "GANG_CONFIG_DIR", "GANG_SESSION", "GANG_STATE_ROOT", "GANG_TMUX_SOCKET", "GANG_COLLARS", "GANG_COLLAR", "GANGLINE_HITCH_ID", "GANG_AGENT_ID", "GANG_AGENT_NONCE", "GANG_AGENT_TOKEN"),
		"GANG_SESSION="+session, "GANG_CONFIG_DIR="+filepath.Join(root, "config"), "GANG_STATE_ROOT="+filepath.Join(root, "state"), "GANG_TMUX_SOCKET="+socket, "GANG_COLLARS="+collars, "GANG_COLLAR=acceptance", "GANGLINE_ACCEPTANCE_REHITCH_HARNESS=1", "GANGLINE_ACCEPTANCE_ROOT="+root, "GANGLINE_ACCEPTANCE_GANG="+binary)
	runner := tmuxRunner{binary: "tmux", socket: socket, env: env}
	runner.newSession(t, session)
	placeholder := strings.TrimSpace(mustResumeTmux(t, runner, "display-message", "-p", "-t", session, "#{pane_id}"))
	t.Cleanup(func() { _, _ = runner.run("kill-session", "-t", session) })
	if got := strings.TrimSpace(mustResumeTmux(t, runner, "list-sessions", "-F", "#{session_name}")); got != session {
		t.Fatalf("unexpected sessions: %s", got)
	}
	runGang := func(args ...string) (string, error) {
		cmd := exec.Command(binary, args...)
		cmd.Env, cmd.Dir = env, repo
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	t.Cleanup(func() {
		if out, err := runGang("roster"); err != nil {
			t.Errorf("cleanup roster: %v: %s", err, out)
		}
		if out, err := runGang("down", "--yes"); err != nil {
			t.Errorf("cleanup team: %v: %s", err, out)
		}
	})
	expect := func(want string) {
		t.Helper()
		got, err := reader.ReadString('\n')
		if err != nil || strings.TrimSpace(got) != want {
			t.Fatalf("event=%q err=%v want=%s", got, err, want)
		}
	}
	if out, err := runGang("hitch", "owner", "--resume", "native-conversation", "-r", "lead", "-t", "initial work"); err != nil {
		t.Fatalf("hitch: %v: %s", err, out)
	}
	expect("received")
	mustResumeTmux(t, runner, "kill-pane", "-t", placeholder)
	team, _ := (store.Paths{Root: filepath.Join(root, "state")}).Team(session)
	id, err := team.ResolveName("owner")
	if err != nil {
		t.Fatal(err)
	}
	p, _ := team.Agent(id)
	before, err := p.Read()
	if err != nil {
		t.Fatal(err)
	}
	// The restart returns the agent to its place between the operator's
	// windows, although closing its window renumbers them and fills the gap
	// the placeholder left.
	mustResumeTmux(t, runner, "set-option", "-t", session, "renumber-windows", "on")
	ownerWindow := strings.TrimSpace(mustResumeTmux(t, runner, "display-message", "-p", "-t", before.Pane, "#{window_id}"))
	mustResumeTmux(t, runner, "new-window", "-d", "-b", "-n", "earlier", "-t", ownerWindow, "cat")
	mustResumeTmux(t, runner, "new-window", "-d", "-a", "-n", "later", "-t", ownerWindow, "cat")
	windows := func() string {
		t.Helper()
		return strings.Join(strings.Fields(mustResumeTmux(t, runner, "list-windows", "-t", session+":", "-F", "#{window_name}")), " ")
	}
	if got := windows(); got != "earlier owner later" {
		t.Fatalf("window setup: %q", got)
	}
	supervisor := core.Agent{ID: "supervisor", Name: "supervisor", Collar: "acceptance", Directory: repo, Status: core.Active}
	supervisorLock, err := team.CreateAgent(supervisor)
	if err != nil {
		t.Fatal(err)
	}
	_ = supervisorLock.Close()
	before.HitchedBy = supervisor.ID
	ownerLock, err := p.LockAgent()
	if err != nil {
		t.Fatal(err)
	}
	if err := ownerLock.Save(before); err != nil {
		t.Fatal(err)
	}
	_ = ownerLock.Close()
	// A child refers to the parent's hitch ID across its restart.
	child := core.Agent{ID: "child", Name: "child", Collar: "acceptance", Directory: repo, Status: core.Active, HitchedBy: id}
	l, err := team.CreateAgent(child)
	if err != nil {
		t.Fatal(err)
	}
	_ = l.Close()
	enter := func(text string) {
		mustResumeTmux(t, runner, "send-keys", "-t", before.Pane, "-l", text)
		mustResumeTmux(t, runner, "send-keys", "-t", before.Pane, "Enter")
	}
	if err := os.WriteFile(filepath.Join(root, "update-fails"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	enter("__REHITCH__")
	expect("update-ran")
	expect("update-failed")
	failed, _ := p.Read()
	if failed.Rehitch != nil || failed.Pane != before.Pane || failed.ID != before.ID {
		t.Fatalf("update failure changed registration: %+v", failed)
	}
	if err := os.Remove(filepath.Join(root, "update-fails")); err != nil {
		t.Fatal(err)
	}
	enter("__REHITCH__")
	expect("update-ran")
	expect("request-queued")
	queued, _ := p.Read()
	if queued.Rehitch == nil || queued.Pane != before.Pane {
		t.Fatalf("request did not wait for finish: %+v", queued)
	}
	pending := core.Envelope{ID: "queued-before-restart", Token: "0123456789abcdef", Recipient: id, To: before.Name, From: core.Sender{Kind: core.SenderSelfDeclared, Name: "operator"}, Message: core.Message{Text: "retained pending work"}, CreatedAt: time.Now(), NotBefore: time.Now().Add(time.Hour)}
	if err := p.Publish(pending); err != nil {
		t.Fatal(err)
	}
	enter("__FINISH__")
	expect("turn-finished")
	nonce, err := os.ReadFile(filepath.Join(root, "nonce"))
	if err != nil {
		t.Fatal(err)
	}
	tick := exec.Command(binary, "tick", "--agent", string(id))
	tick.Env = append(env, "GANG_AGENT_ID="+string(id), "GANGLINE_HITCH_ID="+string(id), "GANG_AGENT_NONCE="+string(nonce), "TMUX_PANE="+before.Pane)
	if out, err := tick.CombinedOutput(); err != nil {
		t.Fatalf("restart tick: %v: %s", err, out)
	}
	expect("received")
	after, err := p.Read()
	if err != nil {
		t.Fatal(err)
	}
	if after.ID != before.ID || after.Pane == before.Pane || after.Role != before.Role || after.HitchedBy != before.HitchedBy || after.Native.SessionID != "native-conversation" || after.Rehitch != nil {
		t.Fatalf("restart lost identity: before=%+v after=%+v", before, after)
	}
	if got := windows(); got != "earlier owner later" {
		t.Fatalf("restart moved the agent's window: %q", got)
	}
	argv, err := os.ReadFile(filepath.Join(root, string(id)+".argv"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(argv), "--resume\nnative-conversation\n") {
		t.Fatalf("native resume identity absent: %s", argv)
	}
	for _, arg := range strings.Split(string(argv), "\n") {
		if arg == "-m" || arg == "--model" || arg == "-e" || arg == "--effort" {
			t.Fatalf("native resume overrides its conversation settings: %s", argv)
		}
	}
	wire, err := os.ReadFile(filepath.Join(root, string(id)+".received"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(wire), "Assignment: resume saved work") {
		t.Fatalf("continuation absent: %s", wire)
	}
	if receipt, err := p.ReadEnvelope("cur", after.LastDelivered); err != nil || receipt.Outcome != "delivered" || receipt.From.HitchID != id {
		t.Fatalf("continuation receipt: %+v: %v", receipt, err)
	}
	remaining, err := p.ListNew()
	if err != nil || len(remaining) != 1 || remaining[0].ID != pending.ID {
		t.Fatalf("inbox not retained: %+v: %v", remaining, err)
	}
	childPaths, _ := team.Agent(child.ID)
	gotChild, err := childPaths.Read()
	if err != nil || gotChild.HitchedBy != after.ID {
		t.Fatalf("child ownership: %+v: %v", gotChild, err)
	}
	// A native startup exit after teardown keeps failed state and pending mail.
	before.Pane = after.Pane
	enter("__REHITCH__")
	expect("update-ran")
	expect("request-queued")
	if err := os.WriteFile(filepath.Join(root, "resume-exits"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	enter("__FINISH__")
	expect("turn-finished")
	if out, err := runGang("tick", "--agent", string(id)); err == nil || !strings.Contains(out, "synthetic resume startup exit") {
		t.Fatalf("startup failure: %v: %s", err, out)
	}
	expect("resume-exited")
	failedRestart, err := p.Read()
	if err != nil || failedRestart.ID != id || failedRestart.Status != core.Failed || failedRestart.Rehitch != nil || failedRestart.Pane != "" {
		t.Fatalf("failed restart state: %+v: %v", failedRestart, err)
	}
	remaining, err = p.ListNew()
	if err != nil {
		t.Fatal(err)
	}
	retained, noticed := false, false
	for _, e := range remaining {
		retained = retained || e.ID == pending.ID
		noticed = noticed || strings.Contains(e.Message.Text, "could not restart its harness")
	}
	if !retained || !noticed {
		t.Fatalf("failed restart mail: %+v", remaining)
	}
	supervisorPaths, _ := team.Agent(supervisor.ID)
	notices, err := supervisorPaths.ListNew()
	if err != nil {
		t.Fatal(err)
	}
	noticed = false
	for _, e := range notices {
		noticed = noticed || strings.Contains(e.Message.Text, "could not restart its harness")
	}
	if !noticed {
		t.Fatalf("hitcher failure notice absent: %+v", notices)
	}
	if out, err := runGang("tick", "--agent", string(id)); err != nil {
		t.Fatalf("failed-state tick: %v: %s", err, out)
	}
	unchanged, err := p.Read()
	if err != nil || unchanged.Status != core.Failed || unchanged.Rehitch != nil {
		t.Fatalf("failure retried automatically: %+v: %v", unchanged, err)
	}

}

func runRehitchHarness() int {
	root := os.Getenv("GANGLINE_ACCEPTANCE_ROOT")
	if len(os.Args) > 1 && os.Args[1] == "--fixture-update" {
		_ = resumeHarnessEvent("update-ran")
		if _, err := os.Stat(filepath.Join(root, "update-fails")); err == nil {
			fmt.Fprintln(os.Stderr, "synthetic update failure")
			return 9
		}
		fmt.Println("synthetic update completed")
		return 0
	}
	if _, err := os.Stat(filepath.Join(root, "resume-exits")); err == nil {
		fmt.Fprintln(os.Stderr, "synthetic resume startup exit")
		_ = resumeHarnessEvent("resume-exited")
		return 17
	}
	if err := rehitchHarness(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		_ = resumeHarnessEvent("fixture-error: " + err.Error())
		return 1
	}
	return 0
}

func rehitchHarness() error {
	root, id := os.Getenv("GANGLINE_ACCEPTANCE_ROOT"), os.Getenv("GANGLINE_HITCH_ID")
	if err := os.WriteFile(filepath.Join(root, "nonce"), []byte(os.Getenv("GANG_AGENT_NONCE")), 0600); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(root, id+".argv"), []byte(strings.Join(os.Args[1:], "\n")+"\n"), 0600); err != nil {
		return err
	}
	stty := exec.Command("stty", "raw", "-echo")
	stty.Stdin = os.Stdin
	if out, err := stty.CombinedOutput(); err != nil {
		return fmt.Errorf("stty: %w: %s", err, out)
	}
	// A restart removes this harness's pane while the test reads the next
	// harness's events. Surviving the hangup signal makes every run take the
	// path a late signal leaves open: the read fails with EIO, which ends the
	// harness as EOF does instead of reporting a fixture error.
	ossignal.Notify(make(chan os.Signal, 1), syscall.SIGHUP)
	team, _ := (store.Paths{Root: os.Getenv("GANG_STATE_ROOT")}).Team(os.Getenv("GANG_SESSION"))
	p, _ := team.Agent(core.HitchID(id))
	renderResumeComposer("")
	reader := bufio.NewReader(os.Stdin)
	var input strings.Builder
	for {
		value, err := reader.ReadByte()
		if err == io.EOF || errors.Is(err, syscall.EIO) {
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
		switch text {
		case "__REHITCH__":
			args := []string{"rehitch", "--upgrade", "--note", "resume saved work"}
			cmd := exec.Command(os.Getenv("GANGLINE_ACCEPTANCE_GANG"), args...)
			if out, err := cmd.CombinedOutput(); err != nil {
				if !strings.Contains(string(out), "harness update failed; restart was not queued") {
					return fmt.Errorf("request: %w: %s", err, out)
				}
				renderResumeComposer("")
				if err := resumeHarnessEvent("update-failed"); err != nil {
					return err
				}
				continue
			}
			fmt.Print("\x1b[2J\x1b[HWORKING\r\n")
			if err := resumeHarnessEvent("request-queued"); err != nil {
				return err
			}
		case "__FINISH__":
			if err := p.WriteStatusHook(store.StatusHook{Kind: "turn-finished", NativeEvent: "Stop", SessionID: "native-conversation", At: time.Now()}); err != nil {
				return err
			}
			renderResumeComposer("")
			if err := resumeHarnessEvent("turn-finished"); err != nil {
				return err
			}
		default:
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
}
