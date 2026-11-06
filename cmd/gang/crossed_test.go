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

// A message is marked crossed by the recipient's messages to its sender that
// Gangline had not delivered to the sender when it was queued: queued and
// due, accepted into the native queue, or typed without a matching witness.
func TestSendMarksCrossedMessagesUndeliveredAtQueueTime(t *testing.T) {
	f := newStateFixture(t)
	lead := f.add(t, "lead-id", "lead", "codex")
	worker := f.add(t, "worker-id", "worker", "codex")
	now := f.cmd.now()
	wp, err := f.run.team.Agent(worker.ID)
	if err != nil {
		t.Fatal(err)
	}
	lp, err := f.run.team.Agent(lead.ID)
	if err != nil {
		t.Fatal(err)
	}
	// A draft in lead's composer keeps each message queued.
	f.input.screen = screenWithText("READY", "› draft")
	send := func(body string) core.Envelope {
		t.Helper()
		f.env["GANG_AGENT_ID"], f.env["TMUX_PANE"] = string(worker.ID), worker.Pane
		f.cmd.stdin = strings.NewReader(body)
		if err := f.cmd.send([]string{"lead"}); err != nil {
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
	if got := send("nothing pending"); got.Crossed != "" {
		t.Fatalf("crossed with nothing pending: %q", got.Crossed)
	}

	queued := crossingMail(lead, worker, "msg-queued", "1111111111111111", now.Add(-3*time.Second))
	later := crossingMail(lead, worker, "msg-later", "2222222222222222", now.Add(-2*time.Second))
	later.NotBefore = now.Add(time.Hour)
	other := crossingMail(worker, worker, "msg-other", "5555555555555555", now.Add(-time.Second))
	other.From = core.Sender{Kind: core.SenderSelfDeclared, Name: "lead"}
	for _, e := range []core.Envelope{queued, later, other} {
		if err := wp.Publish(e); err != nil {
			t.Fatal(err)
		}
	}
	l, err := wp.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	w, err := wp.Read()
	if err != nil {
		t.Fatal(err)
	}
	accepted := crossingMail(lead, worker, "msg-accepted", "3333333333333333", now.Add(-5*time.Second))
	unverified := crossingMail(lead, worker, "msg-unverified", "4444444444444444", now.Add(-4*time.Second))
	if err := l.Settle(&w, accepted, "accepted", ""); err != nil {
		t.Fatal(err)
	}
	if err := l.Settle(&w, unverified, "unverified", "submit witness does not match"); err != nil {
		t.Fatal(err)
	}
	l.Close()
	got := send("several pending")
	// Oldest first; the scheduled message is not yet due and the
	// self-declared one is not attributable to lead.
	if got.Crossed != "3333333333333333,4444444444444444,1111111111111111" {
		t.Fatalf("crossed = %q", got.Crossed)
	}

	// A delivered message no longer crosses.
	l, err = wp.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	w, _ = wp.Read()
	// A late witness reopens the unverified receipt and settles it delivered.
	if err := f.run.reopenReceipt(l, &w, unverified, "failed"); err != nil {
		t.Fatal(err)
	}
	if err := l.Settle(&w, unverified, "delivered", ""); err != nil {
		t.Fatal(err)
	}
	if err := l.Withdraw(queued.ID); err != nil {
		t.Fatal(err)
	}
	l.Close()
	if got := send("one pending"); got.Crossed != "3333333333333333" {
		t.Fatalf("crossed after delivery = %q", got.Crossed)
	}
	// A witness of the accepted message, not yet reconciled, delivered it.
	wire, err := envelopeText(accepted)
	if err != nil {
		t.Fatal(err)
	}
	if err := wp.WriteWitness(store.Witness{ID: "w1", At: now, Prompt: wire, SessionID: "s"}); err != nil {
		t.Fatal(err)
	}
	if got := send("answers it"); got.Crossed != "" {
		t.Fatalf("answered message still crossed: %q", got.Crossed)
	}
	// An unrelated older witness leaves a new accepted message in flight.
	again := crossingMail(lead, worker, "msg-again", "6666666666666666", now)
	l, err = wp.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	w, _ = wp.Read()
	if err := l.Settle(&w, again, "accepted", ""); err != nil {
		t.Fatal(err)
	}
	l.Close()
	if got := send("still in flight"); got.Crossed != "6666666666666666" {
		t.Fatalf("crossed with an earlier witness = %q", got.Crossed)
	}
	// Once worker has taken later input, an unreconciled receipt is no
	// longer in flight.
	path, _ := wp.EnvelopePath("cur", again.ID)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := wp.WriteWitness(store.Witness{ID: "w2", At: info.ModTime().Add(time.Second), Prompt: "later input", SessionID: "s"}); err != nil {
		t.Fatal(err)
	}
	if got := send("after a turn"); got.Crossed != "" {
		t.Fatalf("stale receipt still crossed: %q", got.Crossed)
	}
	// Unreadable records mark the field unknown and do not stop the send.
	if err := os.WriteFile(filepath.Join(wp.Inbox, "new", "msg-broken.json"), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := send("records unreadable"); got.Crossed != "unknown" {
		t.Fatalf("crossed with unreadable records = %q", got.Crossed)
	}
	if err := os.Remove(filepath.Join(wp.Inbox, "new", "msg-broken.json")); err != nil {
		t.Fatal(err)
	}
	// The marker rides on the stamp line when the message is delivered,
	// whatever has happened since it was queued.
	var submitted []string
	submit := f.input.submit
	f.input.submit = func(prompt string) error {
		submitted = append(submitted, prompt)
		return submit(prompt)
	}
	f.input.screen = screenWithText("READY", "› ")
	if _, err := f.run.drain(lead.ID, ""); err != nil {
		t.Fatal(err)
	}
	want := regexp.MustCompile(`^\[gang:worker#[0-9a-f]{16}\] (.*) \[/gang:worker#[0-9a-f]{16}\]\n\[gang ctx=\S+ at=\S+( crossed=\S+)? /\]$`)
	lines := map[string]string{}
	for _, prompt := range submitted {
		if m := want.FindStringSubmatch(prompt); m != nil {
			lines[m[1]] = m[2]
		}
	}
	for _, body := range []string{"nothing pending", "several pending", "one pending", "answers it", "still in flight", "after a turn", "records unreadable"} {
		if _, ok := lines[body]; !ok {
			t.Fatalf("%q not submitted: %q", body, submitted)
		}
	}
	if lines["nothing pending"] != "" || lines["records unreadable"] != " crossed=unknown" || lines["several pending"] != " crossed=3333333333333333,4444444444444444,1111111111111111" || lines["one pending"] != " crossed=3333333333333333" {
		t.Fatalf("stamp lines: %q from %q", lines, submitted)
	}
}
