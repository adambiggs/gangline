package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/store"
)

// placeholderQuote quotes a token shaped like Claude Code's paste placeholder.
// Claude Code replaces such a token with the text of a live earlier paste
// carrying that number, so the prompt it submits is no longer the message.
const placeholderQuote = "the composer showed [Pasted text #102 +12 lines] after the clear"

func claudeRecipient(t *testing.T) (*stateFixture, core.Agent, store.AgentPaths) {
	t.Helper()
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "claude")
	f.input.command = "claude"
	f.input.screen = screenWithText("────────────────", "❯ ", "────────────────")
	p, err := f.run.team.Agent(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	return f, a, p
}

func requirePlaceholderRefusal(t *testing.T, err error) {
	t.Helper()
	var refused commandError
	if !errors.As(err, &refused) || refused.status != exitRefused || !strings.Contains(err.Error(), "paste placeholder") {
		t.Fatalf("placeholder refusal: %v", err)
	}
}

func requireNothingTyped(t *testing.T, f *stateFixture, p store.AgentPaths) {
	t.Helper()
	pending, err := p.ListNew()
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 0 || f.input.pasted != "" || f.input.submits != 0 || len(f.input.keys) != 0 {
		t.Fatalf("refused input reached the recipient: pending=%d pasted=%q submits=%d keys=%v", len(pending), f.input.pasted, f.input.submits, f.input.keys)
	}
}

func TestSendRefusesPastePlaceholderForClaude(t *testing.T) {
	f, _, p := claudeRecipient(t)
	requirePlaceholderRefusal(t, f.cmd.send([]string{"worker", "--from", "operator", placeholderQuote}))
	requireNothingTyped(t, f, p)
}

// An exact-prompt harness submits the bytes it was given, so the same quote
// is an ordinary message there.
func TestSendDeliversPastePlaceholderToExactPromptHarness(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "codex")
	f.env["GANGLINE_HITCH_ID"] = string(a.ID)
	if err := f.cmd.send([]string{"worker", "--from", "operator", placeholderQuote}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(f.input.pasted, placeholderQuote) || f.input.submits != 1 {
		t.Fatalf("pasted=%q submits=%d", f.input.pasted, f.input.submits)
	}
}

func TestCompactRefusesPastePlaceholderInResumeNote(t *testing.T) {
	f, _, p := claudeRecipient(t)
	requirePlaceholderRefusal(t, f.cmd.compact([]string{"worker", "--resume", placeholderQuote}))
	requireNothingTyped(t, f, p)
	got, err := p.Read()
	if err != nil {
		t.Fatal(err)
	}
	if got.Compaction != nil {
		t.Fatalf("refused compaction recorded: %+v", got.Compaction)
	}
}

func TestInterruptRefusesPastePlaceholderBeforeInterrupting(t *testing.T) {
	f, _, p := claudeRecipient(t)
	requirePlaceholderRefusal(t, f.cmd.interrupt([]string{"worker", "-m", placeholderQuote}))
	requireNothingTyped(t, f, p)
}

func TestHitchRefusesPastePlaceholderInTask(t *testing.T) {
	f := newStateFixture(t)
	t.Setenv("PATH", t.TempDir())
	requirePlaceholderRefusal(t, f.cmd.hitch([]string{"worker", "-c", "claude", "-t", placeholderQuote}))
	agents, err := f.run.team.ListAgents()
	if err != nil {
		t.Fatal(err)
	}
	if len(agents) != 0 {
		t.Fatalf("refused hitch claimed agents: %+v", agents)
	}
}

// A queued envelope that reaches delivery with a placeholder-shaped token
// fails before any text is typed, and the queue moves past it.
func TestDeliveryFailsPastePlaceholderBeforeTyping(t *testing.T) {
	f, a, p := claudeRecipient(t)
	e := core.Envelope{ID: "msg-placeholder", Token: "0123456789abcdef", Recipient: a.ID, To: a.Name, From: core.Sender{Kind: core.SenderGangline, Name: "interrupt"}, Message: core.Message{Text: placeholderQuote}, CreatedAt: f.cmd.now()}
	if err := p.Publish(e); err != nil {
		t.Fatal(err)
	}
	outcome, err := f.run.drain(a.ID, e.ID)
	if err != nil {
		t.Fatal(err)
	}
	requireNothingTyped(t, f, p)
	failed, err := p.ReadEnvelope("failed", e.ID)
	if outcome != "failed" || err != nil || failed.Outcome != "failed" || !strings.Contains(failed.Reason, "paste placeholder") {
		t.Fatalf("outcome=%s envelope=%+v err=%v", outcome, failed, err)
	}
}
