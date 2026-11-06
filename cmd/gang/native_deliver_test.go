//go:build acceptance

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/store"
)

type queueCall struct {
	argv, env []string
}

const (
	turnStarted   = `{"timestamp":"2026-09-22T09:59:00Z","type":"event_msg","payload":{"type":"task_started"}}` + "\n"
	turnCompleted = `{"timestamp":"2026-09-22T09:59:00Z","type":"event_msg","payload":{"type":"task_complete","turn_id":"t0"}}` + "\n"
	turnAborted   = `{"timestamp":"2026-09-22T09:59:00Z","type":"event_msg","payload":{"type":"turn_aborted","reason":"interrupted"}}` + "\n"
)

// codexQueueRecipient binds a Codex recipient to a rollout under its own
// Codex home holding turns, or to a thread with no rollout yet when turns is
// empty.
func codexQueueRecipient(t *testing.T, turns string) (*stateFixture, core.Agent, store.AgentPaths, string) {
	t.Helper()
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "codex")
	p, _ := f.run.team.Agent(a.ID)
	home := filepath.Join(filepath.Dir(f.transcript), "codex-home")
	path := filepath.Join(home, "sessions", "2026", "09", "22", "rollout-2026-09-22T10-00-00-s.jsonl")
	if turns != "" {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(`{"timestamp":"2026-09-22T09:58:00Z","type":"session_meta","payload":{"id":"s"}}`+"\n"+turns), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	a = f.setAgent(t, a, func(a *core.Agent) { a.Native.SessionID, a.Native.Transcript = "s", path })
	// The submit hook records its witness; the fixture's own hook would wait
	// on the agent lock that delivery holds.
	witnesses := 0
	f.input.submit = func(prompt string) error {
		witnesses++
		return p.WriteWitness(store.Witness{ID: fmt.Sprintf("w%d", witnesses), At: f.cmd.now(), SessionID: "s", TurnID: fmt.Sprintf("t%d", witnesses), Transcript: path, Prompt: prompt})
	}
	f.input.screen = screenWithText("› ")
	return f, a, p, home
}

func deliverDirect(t *testing.T, f *stateFixture, a core.Agent, p store.AgentPaths, e core.Envelope) string {
	t.Helper()
	if err := p.Publish(e); err != nil {
		t.Fatal(err)
	}
	f.run.cmd = f.cmd
	c, err := loadCollar("codex", f.run.settings)
	if err != nil {
		t.Fatal(err)
	}
	l, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	outcome, _, err := f.run.deliver(l, &a, e, f.input, c, "")
	if err != nil {
		t.Fatal(err)
	}
	return outcome
}

func operatorMessage(a core.Agent, f *stateFixture, text string) core.Envelope {
	return core.Envelope{ID: core.EnvelopeID("m-" + text), Token: "0123456789abcdef", Recipient: a.ID, To: a.Name, From: core.Sender{Kind: core.SenderSelfDeclared, Name: "operator"}, Message: core.Message{Text: text}, CreatedAt: f.cmd.now()}
}

func TestCodexQueueDeliversWithoutTyping(t *testing.T) {
	f, a, p, home := codexQueueRecipient(t, turnStarted+turnCompleted)
	var calls []queueCall
	f.cmd.nativeQueue = func(_ context.Context, argv, env []string) ([]byte, error) {
		calls = append(calls, queueCall{argv, env})
		// Codex prints warnings on success; only the exit status counts.
		return []byte("WARNING: proceeding\nQueued message q1 for thread s.\n"), nil
	}
	e := operatorMessage(a, f, "queued")
	if outcome := deliverDirect(t, f, a, p, e); outcome != "accepted" {
		t.Fatalf("outcome = %s", outcome)
	}
	if len(calls) != 1 || f.input.submits != 0 || f.input.pasted != "" || len(f.input.keys) != 0 {
		t.Fatalf("calls=%d submits=%d pasted=%q keys=%v", len(calls), f.input.submits, f.input.pasted, f.input.keys)
	}
	argv := calls[0].argv
	if len(argv) != 6 || argv[0] != "codex" || argv[1] != "queue" || argv[2] != "--thread" || argv[3] != "s" || argv[4] != "--message" || !strings.Contains(argv[5], "queued") || !strings.HasPrefix(argv[5], "[gang:") {
		t.Fatalf("argv = %q", argv)
	}
	if env := calls[0].env; env[len(env)-1] != "CODEX_HOME="+home {
		t.Fatalf("CODEX_HOME = %q, want %q", env[len(env)-1], home)
	}
	got, err := p.ReadEnvelope("cur", e.ID)
	if err != nil || got.Outcome != "accepted" || !strings.Contains(got.Reason, "do not resend") {
		t.Fatalf("receipt = %+v %v", got, err)
	}
	// The queued input fires the submit hook with the exact wire.
	if err := f.input.submit(argv[5]); err != nil {
		t.Fatal(err)
	}
	if err := f.run.tickAgent(a.ID, hookNotice{}, false); err != nil {
		t.Fatal(err)
	}
	got, err = p.ReadEnvelope("cur", e.ID)
	if err != nil || got.Outcome != "delivered" || f.input.submits != 0 {
		t.Fatalf("after hook: %+v %v submits=%d", got, err, f.input.submits)
	}
}

func TestCodexQueueAcceptanceStaysAcceptedWithoutItsHook(t *testing.T) {
	f, a, p, _ := codexQueueRecipient(t, turnStarted+turnCompleted)
	f.cmd.nativeQueue = func(context.Context, []string, []string) ([]byte, error) { return nil, nil }
	e := operatorMessage(a, f, "unseen")
	if outcome := deliverDirect(t, f, a, p, e); outcome != "accepted" {
		t.Fatalf("outcome = %s", outcome)
	}
	if err := f.input.submit("another prompt"); err != nil {
		t.Fatal(err)
	}
	if err := f.run.tickAgent(a.ID, hookNotice{}, false); err != nil {
		t.Fatal(err)
	}
	got, err := p.ReadEnvelope("cur", e.ID)
	if err != nil || got.Outcome != "accepted" || f.input.submits != 0 {
		t.Fatalf("receipt = %+v %v submits=%d", got, err, f.input.submits)
	}
}

func TestCodexQueueRefusalTypesTheMessage(t *testing.T) {
	exit := exec.Command("sh", "-c", "exit 1").Run()
	for name, err := range map[string]error{
		"non-zero exit":   exit,
		"missing command": &exec.Error{Name: "codex", Err: exec.ErrNotFound},
	} {
		t.Run(name, func(t *testing.T) {
			f, a, p, _ := codexQueueRecipient(t, turnStarted+turnCompleted)
			calls := 0
			f.cmd.nativeQueue = func(context.Context, []string, []string) ([]byte, error) {
				calls++
				return []byte("Error: failed to queue session message\n"), err
			}
			e := operatorMessage(a, f, "typed")
			if outcome := deliverDirect(t, f, a, p, e); outcome != "delivered" {
				t.Fatalf("outcome = %s", outcome)
			}
			got, rerr := p.ReadEnvelope("cur", e.ID)
			if rerr != nil || calls != 1 || f.input.submits != 1 || !strings.Contains(got.Reason, "typed instead") {
				t.Fatalf("receipt = %+v %v calls=%d submits=%d", got, rerr, calls, f.input.submits)
			}
		})
	}
}

func TestCodexQueueThatMayHoldTheMessageIsNotTyped(t *testing.T) {
	f, a, p, _ := codexQueueRecipient(t, turnStarted+turnCompleted)
	f.cmd.nativeQueue = func(context.Context, []string, []string) ([]byte, error) {
		return nil, context.DeadlineExceeded
	}
	e := operatorMessage(a, f, "uncertain")
	if outcome := deliverDirect(t, f, a, p, e); outcome != "unverified" {
		t.Fatalf("outcome = %s", outcome)
	}
	got, err := p.ReadEnvelope("failed", e.ID)
	if err != nil || f.input.submits != 0 || !strings.Contains(got.Reason, "may hold the message") || !strings.Contains(got.Reason, "do not resend") {
		t.Fatalf("receipt = %+v %v submits=%d", got, err, f.input.submits)
	}
}

func TestCodexQueueWaitsForACompletedTurn(t *testing.T) {
	for name, turns := range map[string]string{
		"no rollout":     "",
		"no turn":        "\n",
		"running turn":   turnStarted + turnCompleted + turnStarted,
		"interrupted":    turnStarted + turnAborted,
		"failed turn":    turnStarted + `{"timestamp":"2026-09-22T09:59:00Z","type":"event_msg","payload":{"type":"task_complete","error":{"message":"overloaded"}}}` + "\n",
		"partial record": turnStarted + turnCompleted + turnStarted[:20],
	} {
		t.Run(name, func(t *testing.T) {
			f, a, p, _ := codexQueueRecipient(t, turns)
			f.cmd.nativeQueue = func(context.Context, []string, []string) ([]byte, error) {
				return nil, errors.New("queue called without a completed turn")
			}
			e := operatorMessage(a, f, "typed")
			if outcome := deliverDirect(t, f, a, p, e); outcome != "delivered" || f.input.submits != 1 {
				t.Fatalf("outcome = %s submits=%d", outcome, f.input.submits)
			}
		})
	}
}

func TestCodexQueueCarriesOnlyOrdinaryMessages(t *testing.T) {
	gangline := func(name string) core.Sender {
		return core.Sender{Kind: core.SenderGangline, Name: core.AgentName(name)}
	}
	for name, change := range map[string]func(*core.Envelope){
		"startup":      func(e *core.Envelope) { e.Purpose = "startup" },
		"assignment":   func(e *core.Envelope) { e.Purpose = "assignment" },
		"context band": func(e *core.Envelope) { e.From = gangline("context-band") },
		"resume":       func(e *core.Envelope) { e.Purpose = "resume" },
		"capacity":     func(e *core.Envelope) { e.From = gangline("capacity-recovery") },
	} {
		t.Run(name, func(t *testing.T) {
			f, a, p, _ := codexQueueRecipient(t, turnStarted+turnCompleted)
			f.cmd.nativeQueue = func(context.Context, []string, []string) ([]byte, error) {
				t.Fatal("native queue carried a " + name + " message")
				return nil, nil
			}
			e := operatorMessage(a, f, "special")
			change(&e)
			deliverDirect(t, f, a, p, e)
			// A capacity continuation without its failed turn is withheld
			// before Enter; it still never reaches the native queue.
			if name != "capacity" && f.input.submits != 1 {
				t.Fatalf("submits = %d", f.input.submits)
			}
		})
	}
}
