//go:build acceptance

package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/store"
)

func crossingMail(from core.Agent, to core.Agent, id, token string, at time.Time) core.Envelope {
	return core.Envelope{ID: core.EnvelopeID(id), Token: token, From: core.Sender{Kind: core.SenderAgent, Name: from.Name, HitchID: from.ID}, Recipient: to.ID, To: to.Name, Message: core.Message{Text: "note"}, CreatedAt: at}
}

// crossingFixture sends from one agent to lead while a draft in lead's
// composer keeps each message queued, and records lead's messages to that
// agent as gang send would.
type crossingFixture struct {
	*stateFixture
	lead core.Agent
}

func (f crossingFixture) send(t *testing.T, from core.Agent, body string) core.Envelope {
	t.Helper()
	f.env["GANG_AGENT_ID"], f.env["TMUX_PANE"] = string(from.ID), from.Pane
	f.cmd.stdin = strings.NewReader(body)
	// Every fixture agent shares one pane: it shows the sender's harness
	// while gang checks the sender, then lead's for delivery, which lead's
	// draft holds.
	f.input.command = from.Collar
	f.input.onForeground = func() { f.input.onForeground, f.input.command = func() { f.input.command = "codex" }, from.Collar }
	defer func() { f.input.command, f.input.onForeground = "codex", nil }()
	if err := f.cmd.send([]string{"lead"}); err != nil {
		t.Fatal(err)
	}
	lp, err := f.run.team.Agent(f.lead.ID)
	if err != nil {
		t.Fatal(err)
	}
	queued, err := lp.ListNew()
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range queued {
		if e.Message.Text == body {
			return e
		}
	}
	t.Fatalf("sent %q not queued", body)
	return core.Envelope{}
}

func (f crossingFixture) logged(t *testing.T, events ...core.Event) {
	t.Helper()
	for _, e := range events {
		if e.At.IsZero() {
			e.At = f.cmd.now()
		}
		if err := f.run.team.Append(e); err != nil {
			t.Fatal(err)
		}
	}
}

func queuedEvent(e core.Envelope) core.Event {
	return core.Event{Type: "send_queued", At: e.CreatedAt, HitchID: e.Recipient, Name: e.To, Envelope: &e}
}

func (f crossingFixture) bindTranscript(t *testing.T, a core.Agent, path string) {
	t.Helper()
	p, err := f.run.team.Agent(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	l, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	b, err := p.Read()
	if err != nil {
		t.Fatal(err)
	}
	b.Native.SessionID, b.Native.Transcript = "s", path
	if err := l.Save(b); err != nil {
		t.Fatal(err)
	}
}

func appendTranscript(t *testing.T, path string, lines ...string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.WriteString(strings.Join(lines, "\n") + "\n"); err != nil {
		t.Fatal(err)
	}
}

func wireJSON(t *testing.T, e core.Envelope) string {
	t.Helper()
	wire, err := envelopeText(e)
	if err != nil {
		t.Fatal(err)
	}
	return strings.ReplaceAll(wire, "\n", `\n`)
}

// A message is marked crossed by the recipient's messages to its sender
// whose envelopes were not yet in the sender's native transcript when it was
// queued.
func TestSendMarksMessagesTheSenderHadNotSeen(t *testing.T) {
	f := crossingFixture{stateFixture: newStateFixture(t)}
	f.lead = f.add(t, "lead-id", "lead", "codex")
	worker := f.add(t, "worker-id", "worker", "claude")
	other := f.add(t, "other-id", "other", "claude")
	now := f.cmd.now()
	f.input.screen = screenWithText("READY", "› draft")

	// With nothing sent to worker, no transcript is needed.
	if got := f.send(t, worker, "nothing pending"); got.Crossed != "" {
		t.Fatalf("crossed with nothing pending: %q", got.Crossed)
	}

	at := func(d time.Duration) time.Time { return now.Add(-d) }
	turn := crossingMail(f.lead, worker, "msg-turn", "1111111111111111", at(5*time.Minute))
	mid := crossingMail(f.lead, worker, "msg-mid", "2222222222222222", at(4*time.Minute))
	enqueued := crossingMail(f.lead, worker, "msg-enqueued", "3333333333333333", at(3*time.Minute))
	queued := crossingMail(f.lead, worker, "msg-queued", "4444444444444444", at(20*time.Minute))
	scheduled := crossingMail(f.lead, worker, "msg-scheduled", "5555555555555555", at(2*time.Minute))
	scheduled.NotBefore = now.Add(time.Hour)
	failed := crossingMail(f.lead, worker, "msg-failed", "6666666666666666", at(2*time.Minute))
	cancelled := crossingMail(f.lead, worker, "msg-cancelled", "7777777777777777", at(2*time.Minute))
	earlier := crossingMail(f.lead, worker, "msg-earlier", "8888888888888888", at(30*time.Minute))
	declared := crossingMail(f.lead, worker, "msg-declared", "9999999999999999", at(time.Minute))
	declared.From = core.Sender{Kind: core.SenderSelfDeclared, Name: "lead"}
	elsewhere := crossingMail(f.lead, other, "msg-elsewhere", "aaaaaaaaaaaaaaaa", at(time.Minute))
	wp, err := f.run.team.Agent(worker.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range []core.Envelope{queued, scheduled} {
		if err := wp.Publish(e); err != nil {
			t.Fatal(err)
		}
	}
	for _, e := range []core.Envelope{earlier, queued, turn, mid, enqueued, scheduled, failed, cancelled, declared, elsewhere} {
		f.logged(t, queuedEvent(e))
	}
	f.logged(t,
		core.Event{Type: "delivery_failed", HitchID: worker.ID, ID: string(failed.ID), Reason: "expired"},
		core.Event{Type: "send_cancelled", HitchID: worker.ID, ID: string(cancelled.ID), Reason: "sender cleared scheduled delivery"})

	// worker's session began after earlier was sent. It took turn as a
	// prompt and mid during a running turn; enqueued is only in its queue,
	// and other records name tokens the model never received as input.
	transcript := filepath.Join(t.TempDir(), "worker.jsonl")
	appendTranscript(t, transcript,
		`{"type":"user","timestamp":"`+at(10*time.Minute).Format(time.RFC3339)+`","message":{"role":"user","content":"start"}}`,
		`{"type":"user","timestamp":"`+at(5*time.Minute).Format(time.RFC3339)+`","message":{"role":"user","content":"`+wireJSON(t, turn)+`"}}`,
		`{"type":"queue-operation","operation":"enqueue","timestamp":"`+at(4*time.Minute).Format(time.RFC3339)+`","content":"`+wireJSON(t, mid)+`"}`,
		`{"type":"attachment","timestamp":"`+at(4*time.Minute).Format(time.RFC3339)+`","attachment":{"type":"queued_command","prompt":"`+wireJSON(t, mid)+`"}}`,
		`{"type":"queue-operation","operation":"enqueue","timestamp":"`+at(3*time.Minute).Format(time.RFC3339)+`","content":"`+wireJSON(t, enqueued)+`"}`,
		`{"type":"user","timestamp":"`+at(2*time.Minute).Format(time.RFC3339)+`","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"t","content":"`+wireJSON(t, queued)+`"}]}}`,
		`{"type":"last-prompt","lastPrompt":"`+wireJSON(t, queued)+`"}`)

	// Without a bound transcript, worker's view cannot be read.
	if got := f.send(t, worker, "no transcript"); got.Crossed != "unknown" {
		t.Fatalf("crossed without a transcript = %q", got.Crossed)
	}
	f.bindTranscript(t, worker, transcript+".missing")
	if got := f.send(t, worker, "transcript missing"); got.Crossed != "unknown" {
		t.Fatalf("crossed with a missing transcript = %q", got.Crossed)
	}
	f.bindTranscript(t, worker, transcript)
	// Oldest first: queued is still in worker's queue, though older than
	// the transcript, and enqueued waits in the harness's queue. The rest
	// were seen, are not yet due, failed, were cancelled, predate the
	// session, are not attributable to lead, or went to another agent.
	if got := f.send(t, worker, "several unseen"); got.Crossed != "4444444444444444,3333333333333333" {
		t.Fatalf("crossed = %q", got.Crossed)
	}
	// The queued prompt reaches the model.
	appendTranscript(t, transcript, `{"type":"user","timestamp":"`+at(time.Minute).Format(time.RFC3339)+`","message":{"role":"user","content":"`+wireJSON(t, enqueued)+`"}}`)
	if got := f.send(t, worker, "one unseen"); got.Crossed != "4444444444444444" {
		t.Fatalf("crossed after the queued prompt ran = %q", got.Crossed)
	}
	// Once it leaves the queue, a message older than the transcript is not
	// judged: its delivery may lie in an earlier session.
	l, err := wp.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Withdraw(queued.ID); err != nil {
		t.Fatal(err)
	}
	l.Close()
	if got := f.send(t, worker, "all seen"); got.Crossed != "" {
		t.Fatalf("crossed after delivery = %q", got.Crossed)
	}
	// worker clears into a new session. Its submit witness names the new
	// session, which worker's old transcript shows continuing it, so the new
	// transcript is read though worker's binding still names the old one.
	cleared := filepath.Join(filepath.Dir(transcript), "s2.jsonl")
	appendTranscript(t, transcript, `{"type":"continued-in","sessionId":"s","continuedInSessionId":"s2"}`)
	moved := crossingMail(f.lead, worker, "msg-moved", "eeeeeeeeeeeeeeee", at(30*time.Second))
	f.logged(t, queuedEvent(moved))
	appendTranscript(t, cleared,
		`{"type":"user","sessionId":"s2","timestamp":"`+at(time.Minute).Format(time.RFC3339)+`","message":{"role":"user","content":"cleared"}}`,
		`{"type":"user","sessionId":"s2","timestamp":"`+at(30*time.Second).Format(time.RFC3339)+`","message":{"role":"user","content":"`+wireJSON(t, moved)+`"}}`)
	if err := wp.WriteWitness(store.Witness{ID: "w", At: at(30 * time.Second), Prompt: "x", SessionID: "s2", Transcript: cleared}); err != nil {
		t.Fatal(err)
	}
	if got := f.send(t, worker, "after clear"); got.Crossed != "" {
		t.Fatalf("crossed after a session move = %q", got.Crossed)
	}
	// A record that could hold an envelope but cannot be read leaves the
	// answer unknown.
	appendTranscript(t, cleared, `{"type":"user","message":"[gang:lead#`)
	if got := f.send(t, worker, "transcript unreadable"); got.Crossed != "unknown" {
		t.Fatalf("crossed with an unreadable transcript = %q", got.Crossed)
	}

	// Codex records input when it enters the conversation, at a turn's
	// start or after the step running when it was typed.
	coder := f.add(t, "coder-id", "coder", "codex")
	read := crossingMail(f.lead, coder, "msg-read", "bbbbbbbbbbbbbbbb", at(3*time.Minute))
	steered := crossingMail(f.lead, coder, "msg-steered", "cccccccccccccccc", at(2*time.Minute))
	pending := crossingMail(f.lead, coder, "msg-pending", "dddddddddddddddd", at(time.Minute))
	f.logged(t, queuedEvent(read), queuedEvent(steered), queuedEvent(pending))
	rollout := filepath.Join(t.TempDir(), "rollout.jsonl")
	appendTranscript(t, rollout,
		`{"timestamp":"`+at(10*time.Minute).Format(time.RFC3339)+`","type":"session_meta","payload":{"id":"s"}}`,
		`{"timestamp":"`+at(3*time.Minute).Format(time.RFC3339)+`","type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"`+wireJSON(t, read)+`"}]}}`,
		`{"timestamp":"`+at(2*time.Minute).Format(time.RFC3339)+`","type":"response_item","payload":{"type":"function_call_output","output":"`+wireJSON(t, pending)+`"}}`,
		`{"timestamp":"`+at(2*time.Minute).Format(time.RFC3339)+`","type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"`+wireJSON(t, steered)+`"}]}}`)
	f.bindTranscript(t, coder, rollout)
	if got := f.send(t, coder, "codex unseen"); got.Crossed != "dddddddddddddddd" {
		t.Fatalf("codex crossed = %q", got.Crossed)
	}

	// An agent whose binding names no transcript yet is read from its
	// submit witness, as delivery reads it.
	fresh := f.add(t, "fresh-id", "fresh", "claude")
	greeted := crossingMail(f.lead, fresh, "msg-greeted", "ffffffffffffffff", at(time.Minute))
	f.logged(t, queuedEvent(greeted))
	first := filepath.Join(t.TempDir(), "f.jsonl")
	appendTranscript(t, first, `{"type":"user","sessionId":"f","timestamp":"`+at(2*time.Minute).Format(time.RFC3339)+`","message":{"role":"user","content":"start"}}`,
		`{"type":"user","sessionId":"f","timestamp":"`+at(time.Minute).Format(time.RFC3339)+`","message":{"role":"user","content":"`+wireJSON(t, greeted)+`"}}`)
	fp, err := f.run.team.Agent(fresh.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := fp.WriteWitness(store.Witness{ID: "w", At: at(time.Minute), Prompt: "x", SessionID: "f", Transcript: first}); err != nil {
		t.Fatal(err)
	}
	if got := f.send(t, fresh, "first session"); got.Crossed != "" {
		t.Fatalf("crossed from the witness's session = %q", got.Crossed)
	}

	// The marker rides on the stamp line when the message is delivered.
	var submitted []string
	submit := f.input.submit
	f.input.submit = func(prompt string) error {
		submitted = append(submitted, prompt)
		return submit(prompt)
	}
	f.input.screen = screenWithText("READY", "› ")
	if _, err := f.run.drain(f.lead.ID, ""); err != nil {
		t.Fatal(err)
	}
	want := regexp.MustCompile(`^\[gang:(?:worker|coder|fresh)#[0-9a-f]{16}\] (.*) \[/gang:(?:worker|coder|fresh)#[0-9a-f]{16}\]\n\[gang ctx=\S+ at=\S+( crossed=\S+)? /\]$`)
	lines := map[string]string{}
	for _, prompt := range submitted {
		if m := want.FindStringSubmatch(prompt); m != nil {
			lines[m[1]] = m[2]
		}
	}
	for body, crossed := range map[string]string{
		"nothing pending":       "",
		"no transcript":         " crossed=unknown",
		"transcript missing":    " crossed=unknown",
		"several unseen":        " crossed=4444444444444444,3333333333333333",
		"one unseen":            " crossed=4444444444444444",
		"all seen":              "",
		"after clear":           "",
		"first session":         "",
		"transcript unreadable": " crossed=unknown",
		"codex unseen":          " crossed=dddddddddddddddd",
	} {
		if got, ok := lines[body]; !ok || got != crossed {
			t.Errorf("%q stamp marker %q (submitted %v), want %q", body, got, ok, crossed)
		}
	}
}
