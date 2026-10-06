package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
)

func publishPending(t *testing.T, f *stateFixture, a core.Agent, e core.Envelope) {
	t.Helper()
	paths, err := f.run.team.Agent(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	e.Recipient, e.To = a.ID, a.Name
	if e.From.Kind == "" {
		e.From = core.Sender{Kind: core.SenderAgent, Name: "lead"}
	}
	if e.CreatedAt.IsZero() {
		e.CreatedAt = f.cmd.now()
	}
	if err := paths.Publish(e); err != nil {
		t.Fatal(err)
	}
}

func saveAgent(t *testing.T, f *stateFixture, id core.HitchID, change func(*core.Agent)) {
	t.Helper()
	p, err := f.run.team.Agent(id)
	if err != nil {
		t.Fatal(err)
	}
	l, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	a, err := p.Read()
	if err != nil {
		t.Fatal(err)
	}
	change(&a)
	if err := l.Save(a); err != nil {
		t.Fatal(err)
	}
}

func decodeOutput(t *testing.T, f *stateFixture, into any) {
	t.Helper()
	decoder := json.NewDecoder(f.out)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(into); err != nil {
		t.Fatalf("decode %q: %v", f.out.String(), err)
	}
	if decoder.More() {
		t.Fatalf("output holds more than one JSON value: %q", f.out.String())
	}
}

func queueJSONRows(t *testing.T, f *stateFixture, args ...string) map[core.EnvelopeID]queueRow {
	t.Helper()
	f.out.Reset()
	if err := f.cmd.execute(append([]string{"queue", "--json"}, args...)); err != nil {
		t.Fatal(err)
	}
	var got queueJSON
	decodeOutput(t, f, &got)
	rows := map[core.EnvelopeID]queueRow{}
	for _, row := range got.Messages {
		rows[row.ID] = row
	}
	return rows
}

func TestQueueCommandListsPendingMessages(t *testing.T) {
	f := newStateFixture(t)
	worker := f.add(t, "a", "worker", "codex")
	other := f.add(t, "b", "other", "codex")
	publishPending(t, f, worker, core.Envelope{ID: "first", From: core.Sender{Kind: core.SenderSelfDeclared, Name: "lead"}, Message: core.Message{Text: "pending\n\twork " + strings.Repeat("x", 200)}})
	publishPending(t, f, other, core.Envelope{ID: "second", Purpose: "assignment", Message: core.Message{Text: "pending"}})
	if err := f.cmd.execute([]string{"queue", "worker"}); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(f.out.String(), "\n"), "\n")
	if len(lines) != 2 || strings.Fields(lines[0])[0] != "ID" {
		t.Fatalf("targeted queue = %q", f.out.String())
	}
	fields := strings.Fields(lines[1])
	if len(fields) < 7 || fields[0] != "first" || fields[1] != "worker" || fields[2] != "self-declared:lead" || fields[3] != "message" || fields[4] != "ready" || !strings.HasPrefix(strings.Join(fields[6:], " "), "pending work xxx") || !strings.HasSuffix(lines[1], "…") {
		t.Fatalf("targeted queue row = %q", lines[1])
	}
	rows := queueJSONRows(t, f)
	if len(rows) != 2 || rows["second"].To != "other" || rows["second"].Kind != "assignment" || rows["second"].From != (core.Sender{Kind: core.SenderAgent, Name: "lead"}) || rows["first"].HitchID != "a" {
		t.Fatalf("team queue = %+v", rows)
	}
	if err := f.cmd.execute([]string{"queue", "worker", "other"}); err == nil || !strings.Contains(err.Error(), "expected at most one agent") {
		t.Fatalf("extra queue argument: %v", err)
	}
}

func TestQueueCommandShowsWhyEachMessageWaits(t *testing.T) {
	f := newStateFixture(t)
	worker := f.add(t, "a", "worker", "codex")
	now := f.cmd.now()
	// The inbox orders by creation time; these arrive in the order listed.
	for i, e := range []core.Envelope{
		{ID: "expired", NotAfter: now.Add(-time.Minute), Message: core.Message{Text: "late"}},
		{ID: "scheduled", NotBefore: now.Add(5 * time.Minute), Message: core.Message{Text: "later"}},
		{ID: "due", Message: core.Message{Text: "now"}},
		{ID: "resume-c1", Purpose: "resume", Message: core.Message{Text: "resume"}},
		{ID: "after-resume", Message: core.Message{Text: "after"}},
	} {
		e.CreatedAt = now.Add(time.Duration(i-10) * time.Second)
		publishPending(t, f, worker, e)
	}
	rows := queueJSONRows(t, f, "worker")
	want := map[core.EnvelopeID]string{"expired": "blocked", "scheduled": "scheduled", "due": "ready", "resume-c1": "blocked", "after-resume": "blocked"}
	for id, state := range want {
		if rows[id].State != state {
			t.Fatalf("%s state = %q, want %q: %+v", id, rows[id].State, state, rows)
		}
	}
	if due := rows["scheduled"].DueAt; due == nil || !due.Equal(now.Add(5*time.Minute)) {
		t.Fatalf("scheduled due_at = %v", due)
	}
	if !strings.Contains(rows["after-resume"].Reason, "resume-c1") || !strings.Contains(rows["expired"].Reason, "expired") {
		t.Fatalf("reasons = %+v", rows)
	}

	f.input.screen = screenWithText("READY", "› unsent draft")
	if row := queueJSONRows(t, f, "worker")["due"]; row.State != "blocked" || row.Reason == "" {
		t.Fatalf("draft in composer: %+v", row)
	}
	f.input.screen = screenWithText("READY", "› ")

	saveAgent(t, f, worker.ID, func(a *core.Agent) { a.Status = core.Failed })
	if row := queueJSONRows(t, f, "worker")["due"]; row.State != "blocked" || !strings.Contains(row.Reason, "failed") {
		t.Fatalf("failed recipient: %+v", row)
	}
	if submits, pasted := f.input.submits, f.input.pasted; submits != 0 || pasted != "" {
		t.Fatalf("queue delivered input: submits=%d pasted=%q", submits, pasted)
	}
}

func TestQueueCommandStartsATickForDueMessages(t *testing.T) {
	f := newStateFixture(t)
	worker := f.add(t, "a", "worker", "codex")
	var detached []string
	f.cmd.detach = func(id string, _ hookNotice) error { detached = append(detached, id); return nil }
	publishPending(t, f, worker, core.Envelope{ID: "due", Message: core.Message{Text: "now"}})
	queueJSONRows(t, f, "worker")
	if len(detached) != 1 || detached[0] != "a" {
		t.Fatalf("queue left a due message without a tick: %v", detached)
	}
}

func TestQueueCommandReportsHeadAndStartupBlocks(t *testing.T) {
	f := newStateFixture(t)
	worker := f.add(t, "a", "worker", "codex")
	now := f.cmd.now()
	for i, id := range []core.EnvelopeID{"first", "second"} {
		publishPending(t, f, worker, core.Envelope{ID: id, CreatedAt: now.Add(time.Duration(i-10) * time.Second), Message: core.Message{Text: "now"}})
	}
	rows := queueJSONRows(t, f, "worker")
	if rows["first"].State != "ready" || rows["first"].Reason != "" || rows["second"].State != "ready" || rows["second"].Reason != "after first" {
		t.Fatalf("two due messages: %+v", rows)
	}

	p, err := f.run.team.Agent(worker.ID)
	if err != nil {
		t.Fatal(err)
	}
	l, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	rows = queueJSONRows(t, f, "worker")
	l.Close()
	if rows["first"].State != "unknown" || !strings.Contains(rows["first"].Reason, "locked") {
		t.Fatalf("locked recipient: %+v", rows)
	}

	f2 := newStateFixture(t)
	held := f2.add(t, "a", "worker", "codex")
	p2, err := f2.run.team.Agent(held.ID)
	if err != nil {
		t.Fatal(err)
	}
	holdStartup(t, f2, &held, p2, "startup")
	publishPending(t, f2, held, core.Envelope{ID: "due", CreatedAt: now.Add(time.Second), Message: core.Message{Text: "now"}})
	if row := queueJSONRows(t, f2, "worker")["due"]; row.State != "blocked" || !strings.Contains(row.Reason, "--recover") {
		t.Fatalf("failed startup: %+v", row)
	}
}

func TestQueueCommandNamesInactiveRecipientBeforeExpiry(t *testing.T) {
	f := newStateFixture(t)
	worker := f.add(t, "a", "worker", "codex")
	publishPending(t, f, worker, core.Envelope{ID: "expired", NotAfter: f.cmd.now().Add(-time.Minute), Message: core.Message{Text: "late"}})
	saveAgent(t, f, worker.ID, func(a *core.Agent) { a.Status = core.Failed })
	if row := queueJSONRows(t, f, "worker")["expired"]; row.State != "blocked" || !strings.Contains(row.Reason, "failed") {
		t.Fatalf("expired message for a failed recipient: %+v", row)
	}
}

func TestQueuePreservesLiteralMessageText(t *testing.T) {
	for _, machine := range []bool{false, true} {
		t.Run(fmt.Sprintf("json=%v", machine), func(t *testing.T) {
			f := newStateFixture(t)
			worker := f.add(t, "a", "worker", "codex")
			const text = "budget $100 and pane %1 are literal input"
			f.input.captureErr = errors.New("pane %1 unavailable")
			publishPending(t, f, worker, core.Envelope{ID: "literal", Message: core.Message{Text: text}})
			args := []string{"queue", "worker"}
			if machine {
				args = append(args, "--json")
			}
			if err := f.cmd.execute(args); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(f.out.String(), "pane worker unavailable") {
				t.Fatalf("diagnostic has no agent name: %q", f.out.String())
			}
			if !strings.Contains(f.out.String(), text) {
				t.Fatalf("message changed: %q", f.out.String())
			}
		})
	}
}
