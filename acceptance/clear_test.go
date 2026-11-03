//go:build acceptance

package acceptance

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/store"
)

func TestSelfClearOnPrivateTmux(t *testing.T) {
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
	for _, mode := range []string{"immediate", "lazy"} {
		t.Run(mode, func(t *testing.T) {
			root := filepath.Join(root, mode)
			if err := os.Mkdir(root, 0700); err != nil {
				t.Fatal(err)
			}
			collars := filepath.Join(root, "collars")
			if err := os.Mkdir(collars, 0700); err != nil {
				t.Fatal(err)
			}
			collar := commandAcceptanceCollar(exe)
			collar = strings.ReplaceAll(collar, "claude-composer", "codex-composer")
			collar = strings.Replace(collar, `payload: {prompt: "prompt"}`, `payload: {prompt: "prompt", session_id: "session_id", transcript_path: "transcript_path"}`, 1)
			collar = strings.Replace(collar, `stop: {event: "turn-finished"}`, `stop: {event: "turn-finished", payload: {session_id: "session_id", transcript_path: "transcript_path"}}
 sessionstart: {event: "session-started", payload: {session_id: "session_id", transcript_path: "transcript_path", source: "source"}}`, 1)
			collar = strings.Replace(collar, `primitives: {`, `primitives: {queue_witness: {name: "codex-pending-input"}`, 1)
			collar = strings.Replace(collar, `actions: {`, `actions: {clear: {text: "/clear", submit: true},`, 1)
			if err := os.WriteFile(filepath.Join(collars, "acceptance.cue"), []byte(collar), 0600); err != nil {
				t.Fatal(err)
			}
			fifo := filepath.Join(root, "events")
			if out, err := exec.Command("mkfifo", fifo).CombinedOutput(); err != nil {
				t.Fatalf("fifo: %v %s", err, out)
			}
			events, err := os.OpenFile(fifo, os.O_RDWR, 0600)
			if err != nil {
				t.Fatal(err)
			}
			defer events.Close()
			reader := bufio.NewReader(events)
			const session = "clear-acceptance"
			socket := filepath.Join(root, "tmux.sock")
			env := append(withoutEnvironment(os.Environ(), "TMUX", "TMUX_PANE", "GANG_CONFIG_DIR", "GANG_SESSION", "GANG_STATE_ROOT", "GANG_TMUX_SOCKET", "GANG_COLLARS", "GANG_COLLAR", "GANGLINE_HITCH_ID", "GANG_AGENT_ID", "GANG_AGENT_NONCE", "GANG_AGENT_TOKEN"),
				"GANG_SESSION="+session, "GANG_CONFIG_DIR="+filepath.Join(root, "config"), "GANG_STATE_ROOT="+filepath.Join(root, "state"), "GANG_TMUX_SOCKET="+socket, "GANG_COLLARS="+collars, "GANG_COLLAR=acceptance", "GANGLINE_ACCEPTANCE_CLEAR_HARNESS="+mode, "GANGLINE_ACCEPTANCE_ROOT="+root, "GANGLINE_ACCEPTANCE_GANG="+binary)
			runner := tmuxRunner{binary: "tmux", socket: socket, env: env}
			runner.newSession(t, session)
			mustResumeTmux(t, runner, "set-option", "-t", session, "default-size", "240x200")
			mustResumeTmux(t, runner, "resize-window", "-t", session, "-x", "240", "-y", "200")
			t.Cleanup(func() { _, _ = runner.run("kill-session", "-t", session) })
			if got := strings.TrimSpace(mustResumeTmux(t, runner, "list-sessions", "-F", "#{session_name}")); got != session {
				t.Fatalf("sessions: %s", got)
			}
			runGang := func(args ...string) (string, error) {
				cmd := exec.Command(binary, args...)
				cmd.Env, cmd.Dir = env, repo
				out, err := cmd.CombinedOutput()
				return string(out), err
			}
			t.Cleanup(func() {
				if out, err := runGang("roster", "--all"); err != nil {
					t.Errorf("roster: %v %s", err, out)
				}
				if out, err := runGang("down", "--yes"); err != nil {
					t.Errorf("down: %v %s", err, out)
				}
			})
			expect := func(want string) {
				t.Helper()
				got, err := reader.ReadString('\n')
				if err != nil || strings.TrimSpace(got) != want {
					t.Fatalf("event %q err=%v want=%s", got, err, want)
				}
			}
			if out, err := runGang("hitch", "owner", "-r", "lead", "-t", "initial phase"); err != nil {
				t.Fatalf("hitch: %v %s", err, out)
			}
			expect("initial")
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
			child := core.Agent{ID: "child", Name: "child", Collar: "acceptance", Directory: repo, Status: core.Active, HitchedBy: id}
			childLock, err := team.CreateAgent(child)
			if err != nil {
				t.Fatal(err)
			}
			_ = childLock.Close()
			before.HitchedBy = "supervisor"
			l, err := p.LockAgent()
			if err != nil {
				t.Fatal(err)
			}
			if err := l.Save(before); err != nil {
				t.Fatal(err)
			}
			_ = l.Close()
			mustResumeTmux(t, runner, "send-keys", "-t", before.Pane, "-l", "__CLEAR__")
			mustResumeTmux(t, runner, "send-keys", "-t", before.Pane, "Enter")
			expect("queued")
			retained := core.Envelope{ID: "retained", Token: "0123456789abcdef", Recipient: id, To: before.Name, From: core.Sender{Kind: core.SenderAgent, Name: "child", HitchID: child.ID}, Message: core.Message{Text: "child result retained"}, CreatedAt: time.Now()}
			if err := p.Publish(retained); err != nil {
				t.Fatal(err)
			}
			mustResumeTmux(t, runner, "send-keys", "-t", before.Pane, "-l", "__FINISH__")
			mustResumeTmux(t, runner, "send-keys", "-t", before.Pane, "Enter")
			expect("finished")
			nonce, err := os.ReadFile(filepath.Join(root, "nonce"))
			if err != nil {
				t.Fatal(err)
			}
			tick := exec.Command(binary, "tick", "--agent", string(id))
			tick.Env = append(env, "GANGLINE_HITCH_ID="+string(id), "GANG_AGENT_ID="+string(id), "GANG_AGENT_NONCE="+string(nonce), "TMUX_PANE="+before.Pane)
			if out, err := tick.CombinedOutput(); err != nil {
				t.Fatalf("tick: %v %s", err, out)
			}
			expect("cleared")
			expect("continued")
			expect("retained")
			expect("arrival")
			after, err := p.Read()
			if err != nil {
				t.Fatal(err)
			}
			if after.Clear == nil || after.Clear.Status != "completed" || after.Native.SessionID != "new-session" || after.ID != before.ID || after.Pane != before.Pane || after.Process != before.Process || after.Registration != before.Registration || after.HitchedBy != before.HitchedBy {
				t.Fatalf("continuity: before=%+v after=%+v", before, after)
			}
			cp, _ := team.Agent(child.ID)
			gotChild, err := cp.Read()
			if err != nil || gotChild.HitchedBy != id {
				t.Fatalf("child lineage: %+v %v", gotChild, err)
			}
			raw, err := os.ReadFile(filepath.Join(root, "continuation"))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(raw), " role") || strings.Index(string(raw), " role") > strings.Index(string(raw), "next phase at change example revision abc") {
				t.Fatalf("role/note order: %s", raw)
			}
			t.Logf("PASS %s: same process/pane/registration/lineage; new native session; continuation, child result, concurrent arrival", mode)
		})
	}
}

func runClearHarness() int {
	if err := clearHarness(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		_ = resumeHarnessEvent("fixture-error: " + err.Error())
		return 1
	}
	return 0
}
func clearHarness() error {
	root := os.Getenv("GANGLINE_ACCEPTANCE_ROOT")
	binary := os.Getenv("GANGLINE_ACCEPTANCE_GANG")
	if err := os.WriteFile(filepath.Join(root, "nonce"), []byte(os.Getenv("GANG_AGENT_NONCE")), 0600); err != nil {
		return err
	}
	stty := exec.Command("stty", "raw", "-echo")
	stty.Stdin = os.Stdin
	if out, err := stty.CombinedOutput(); err != nil {
		return fmt.Errorf("stty: %w %s", err, out)
	}
	team, _ := (store.Paths{Root: os.Getenv("GANG_STATE_ROOT")}).Team(os.Getenv("GANG_SESSION"))
	p, _ := team.Agent(core.HitchID(os.Getenv("GANGLINE_HITCH_ID")))
	session := "old-session"
	transcript := filepath.Join(root, "transcript")
	if err := os.WriteFile(transcript, nil, 0600); err != nil {
		return err
	}
	hook := func(event, source, prompt string) error {
		raw, _ := json.Marshal(map[string]string{"hook_event_name": event, "source": source, "session_id": session, "transcript_path": transcript, "prompt": prompt})
		cmd := exec.Command(binary, "hook")
		cmd.Stdin = strings.NewReader(string(raw))
		out, err := cmd.CombinedOutput()
		if err != nil || len(out) > 0 {
			return fmt.Errorf("hook %s: %v %s", event, err, out)
		}
		return nil
	}
	if err := hook("SessionStart", "startup", ""); err != nil {
		return err
	}
	render := func(text string) {
		fmt.Print("\x1b[2J\x1b[HREADY\r\n› ", strings.ReplaceAll(text, "\n", "\r\n"), "\r\n")
	}
	render("")
	reader := bufio.NewReader(os.Stdin)
	var input strings.Builder
	first := true
	lazy := false
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
			text := input.String()
			if strings.HasSuffix(text, "\x1b[201~") || text == "/clear" || strings.HasSuffix(text, "]") {
				render(strings.TrimSuffix(strings.TrimPrefix(text, "\x1b[200~"), "\x1b[201~"))
			}
			continue
		}
		text := strings.TrimSuffix(strings.TrimPrefix(input.String(), "\x1b[200~"), "\x1b[201~")
		input.Reset()
		switch text {
		case "__CLEAR__":
			cmd := exec.Command(binary, "clear", "--note", "next phase at change example revision abc")
			if out, err := cmd.CombinedOutput(); err != nil {
				return fmt.Errorf("clear: %w %s", err, out)
			}
			fmt.Print("\x1b[2J\x1b[HWORKING\r\n")
			if err := resumeHarnessEvent("queued"); err != nil {
				return err
			}
		case "__FINISH__":
			if err := p.WriteStatusHook(store.StatusHook{Kind: "turn-finished", SessionID: session, Transcript: transcript, At: time.Now()}); err != nil {
				return err
			}
			render("")
			if err := resumeHarnessEvent("finished"); err != nil {
				return err
			}
		case "/clear":
			session = "new-session"
			lazy = os.Getenv("GANGLINE_ACCEPTANCE_CLEAR_HARNESS") == "lazy"
			if !lazy {
				if err := hook("SessionStart", "clear", ""); err != nil {
					return err
				}
			}
			a, err := p.Read()
			if err != nil {
				return err
			}
			e := core.Envelope{ID: "arrival", Token: "fedcba9876543210", Recipient: a.ID, To: a.Name, From: core.Sender{Kind: core.SenderSelfDeclared, Name: "arrival"}, Message: core.Message{Text: "arrived during clear"}, CreatedAt: time.Now()}
			if err := p.Publish(e); err != nil {
				return err
			}
			render("")
			if err := resumeHarnessEvent("cleared"); err != nil {
				return err
			}
		default:
			if lazy {
				if err := hook("SessionStart", "clear", ""); err != nil {
					return err
				}
				lazy = false
			}
			if err := hook("UserPromptSubmit", "", text); err != nil {
				return err
			}
			event := "retained"
			if first {
				event = "initial"
				first = false
			} else if strings.Contains(text, " clear]") {
				event = "continued"
				if err := os.WriteFile(filepath.Join(root, "continuation"), []byte(text), 0600); err != nil {
					return err
				}
			} else if strings.Contains(text, "arrived during clear") {
				event = "arrival"
			}
			render("")
			if err := resumeHarnessEvent(event); err != nil {
				return err
			}
		}
	}
}
