//go:build acceptance

package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
)

// Exercise the real tick, inbox, envelope and native submit-witness path.
// No native harness, tmux server, sleep or deadline is involved.
func TestContextBandMessageRendersEveryToken(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "codex")
	c, err := harness.EmbeddedCollar("codex")
	if err != nil {
		t.Fatal(err)
	}
	c.ContextBands = map[string][]harness.ContextBand{"*": {{Name: "yellow", At: 0.75, Message: "{{band}}|{{threshold_percent}}|{{used_tokens}}|{{limit_tokens}}|{{used_percent}}|{{model}}|{{agent_name}}|{{compact_command}}"}}}
	used, limit, percent := int64(760), int64(1000), 76.0
	a.Native.Context = core.Reading{Kind: "context", Model: "gpt-test", Status: "observed", Used: &used, Limit: &limit, Percent: &percent}
	if err := f.run.noteContextBands(&a, c); err != nil {
		t.Fatal(err)
	}
	if len(a.ContextBands.Pending) != 1 {
		t.Fatalf("pending notes = %d", len(a.ContextBands.Pending))
	}
	want := "yellow|75|760|1000|76|gpt-test|worker|gang compact --note 'Resume from FILE'"
	if got := a.ContextBands.Pending[0].Envelope.Message.Text; got != want {
		t.Fatalf("rendered note = %q, want %q", got, want)
	}
}

func TestContextBandMessageDoesNotReexpandValues(t *testing.T) {
	used, limit, percent := int64(1), int64(10), 10.0
	r := core.Reading{Model: "{{band}}", Used: &used, Limit: &limit, Percent: &percent}
	a := &core.Agent{Name: "worker"}
	band := harness.ContextBand{Name: "early", At: 0.2, Message: "{{model}}"}
	if got := renderContextBandMessage(band, false, a, r); got != "{{band}}" {
		t.Fatalf("replacement expanded a model value: %q", got)
	}
}

func TestQueuedContextBandSurvivesUnconfirmedCheckpoint(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "codex")
	p, err := f.run.team.Agent(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	l, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	e := core.Envelope{
		ID: "context-1", Token: "0123456789abcdef", From: core.Sender{Kind: core.SenderGangline, Name: "context-band"},
		Recipient: a.ID, To: a.Name, Message: core.Message{Text: "crossing"}, CreatedAt: f.cmd.now().Add(-2 * time.Hour),
	}
	if err := f.run.publishOnce(l, &a, e); err != nil {
		t.Fatal(err)
	}
	checkpoint := f.cmd.now().Add(-time.Hour)
	c, err := harness.EmbeddedCollar("codex")
	if err != nil {
		t.Fatal(err)
	}
	if err := f.run.acceptContextReadings(&a, c, []core.Reading{{Kind: "compaction-checkpoint", At: &checkpoint, Source: "session-log"}}); err != nil {
		t.Fatal(err)
	}
	if err := l.Save(a); err != nil {
		t.Fatal(err)
	}
	if err := f.run.publishContextNotes(l, &a); err != nil {
		t.Fatal(err)
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	f.input.screen = screenWithText("READY", "› draft")
	if _, err := f.run.drain(a.ID, ""); err != nil {
		t.Fatal(err)
	}
	if f.input.submits != 0 {
		t.Fatalf("submitted %d messages while input was blocked", f.input.submits)
	}
	if _, err := p.ReadEnvelope("new", e.ID); err != nil {
		t.Fatalf("notice was cancelled before compaction completed: %v", err)
	}
}

func TestCompletedCompactionClearsEverySpooledContextBand(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "codex")
	p, err := f.run.team.Agent(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	l, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	at := f.cmd.now()
	for _, e := range []core.Envelope{
		{ID: "context-1", Token: "0123456789abcdef", From: core.Sender{Kind: core.SenderGangline, Name: "context-band"}, Recipient: a.ID, To: a.Name, Message: core.Message{Text: "before"}, CreatedAt: at.Add(-time.Hour)},
		{ID: "context-2", Token: "fedcba9876543210", From: core.Sender{Kind: core.SenderGangline, Name: "context-band"}, Recipient: a.ID, To: a.Name, Message: core.Message{Text: "already spooled"}, CreatedAt: at.Add(time.Hour)},
		{ID: "ordinary", Token: "1111111111111111", From: core.Sender{Kind: core.SenderAgent, Name: "lead", HitchID: "lead"}, Recipient: a.ID, To: a.Name, Message: core.Message{Text: "keep"}, CreatedAt: at},
	} {
		if err := f.run.publishOnce(l, &a, e); err != nil {
			t.Fatal(err)
		}
	}
	a.ContextBands.Pending = []core.ContextBandNote{{Band: "yellow", Envelope: core.Envelope{
		ID: "context-3", Token: "3333333333333333", From: core.Sender{Kind: core.SenderGangline, Name: "context-band"},
		Recipient: a.ID, To: a.Name, Message: core.Message{Text: "unpublished crossing"}, CreatedAt: at,
	}}}
	c, err := harness.EmbeddedCollar("codex")
	if err != nil {
		t.Fatal(err)
	}
	if err := f.run.acceptContextReadings(&a, c, []core.Reading{{Kind: "compaction-finished", At: &at, Source: "native-hook"}}); err != nil {
		t.Fatal(err)
	}
	if err := l.Save(a); err != nil {
		t.Fatal(err)
	}
	if err := f.run.publishContextNotes(l, &a); err != nil {
		t.Fatal(err)
	}
	if len(a.ContextBands.Pending) != 0 {
		t.Fatalf("stale publication intents remain: %+v", a.ContextBands.Pending)
	}
	queued, err := p.ListNew()
	if err != nil {
		t.Fatal(err)
	}
	if len(queued) != 1 || queued[0].ID != "ordinary" {
		t.Fatalf("spool after compaction = %+v", queued)
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestCompactionHookClearsBandNoticesBeforeDetachedTick(t *testing.T) {
	f, a, p := compactionFixture(t)
	at := f.cmd.now()
	l, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	a.Compaction = &core.Compaction{ID: "compact", StartedAt: at.Add(-time.Minute), Status: "submitted", Continuation: true}
	a.ContextBands.Pending = []core.ContextBandNote{{Band: "yellow", Envelope: core.Envelope{
		ID: "context-2", Token: "fedcba9876543210", From: core.Sender{Kind: core.SenderGangline, Name: "context-band"},
		Recipient: a.ID, To: a.Name, Message: core.Message{Text: "pending"}, CreatedAt: at,
	}}}
	if err := l.Save(a); err != nil {
		t.Fatal(err)
	}
	e := core.Envelope{ID: "context-1", Token: "0123456789abcdef", From: core.Sender{Kind: core.SenderGangline, Name: "context-band"}, Recipient: a.ID, To: a.Name, Message: core.Message{Text: "spooled"}, CreatedAt: at}
	if err := f.run.publishOnce(l, &a, e); err != nil {
		t.Fatal(err)
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.run.confirmCompactionHook(a.ID, hookNotice{Kind: "compaction-finished", SessionID: "s", At: at.Add(time.Second)}); err != nil {
		t.Fatal(err)
	}
	queued, err := p.ListNew()
	if err != nil || len(queued) != 0 {
		t.Fatalf("stale notices remain between hook and tick: %+v, %v", queued, err)
	}
	got, err := p.Read()
	if err != nil || len(got.ContextBands.Pending) != 0 || got.ContextBands.DiscardQueued {
		t.Fatalf("stale publication intent remains: %+v, %v", got.ContextBands, err)
	}
}

func TestAutoCompactionHookClearsBandNoticeBeforeDetachedTick(t *testing.T) {
	f, a, p := compactionFixture(t)
	f.env["GANGLINE_HITCH_ID"] = string(a.ID)
	e := core.Envelope{ID: "context-1", Token: "0123456789abcdef", From: core.Sender{Kind: core.SenderGangline, Name: "context-band"},
		Recipient: a.ID, To: a.Name, Message: core.Message{Text: "stale"}, CreatedAt: f.cmd.now()}
	if err := p.Publish(e); err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(map[string]string{"hook_event_name": "PostCompact", "session_id": "s"})
	if err != nil {
		t.Fatal(err)
	}
	f.cmd.stdin = strings.NewReader(string(payload))
	if err := f.cmd.handleHook(nil); err != nil {
		t.Fatal(err)
	}
	queued, err := p.ListNew()
	if err != nil || len(queued) != 0 {
		t.Fatalf("auto-compaction left a stale notice before detached tick: %+v, %v", queued, err)
	}
}

func TestCompactionWitnessCancelsNoticeAfterLockRelease(t *testing.T) {
	f, a, p := compactionFixture(t)
	f.env["GANGLINE_HITCH_ID"] = string(a.ID)
	e := core.Envelope{ID: "context-1", Token: "0123456789abcdef", From: core.Sender{Kind: core.SenderGangline, Name: "context-band"},
		Recipient: a.ID, To: a.Name, Message: core.Message{Text: "stale"}, CreatedAt: f.cmd.now()}
	if err := p.Publish(e); err != nil {
		t.Fatal(err)
	}
	l, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(map[string]string{"hook_event_name": "PostCompact", "session_id": "s"})
	if err != nil {
		t.Fatal(err)
	}
	f.cmd.stdin = strings.NewReader(string(payload))
	if err := f.cmd.handleHook(nil); err != nil {
		t.Fatal(err)
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	access, _, err := f.run.acquire(a.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := access.Close(); err != nil {
		t.Fatal(err)
	}
	queued, err := p.ListNew()
	if err != nil || len(queued) != 0 {
		t.Fatalf("locked hook notice survived next access: %+v, %v", queued, err)
	}
}

// Discarding stale context notes must leave an existing retained receipt in
// place: it is what holds later delivery behind unconfirmed input.
func TestCompactionDiscardKeepsRetainedReceipt(t *testing.T) {
	f, a, p := compactionFixture(t)
	at := f.cmd.now()
	retained := core.Envelope{ID: "retained", Token: "00112233445566ff", From: core.Sender{Kind: core.SenderGangline, Name: "other"}, Recipient: a.ID, To: a.Name, Message: core.Message{Text: "unconfirmed"}, CreatedAt: at}
	if err := p.Publish(retained); err != nil {
		t.Fatal(err)
	}
	l, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Settle(&a, retained, "unverified", "native input not yet confirmed"); err != nil {
		t.Fatal(err)
	}
	a.Compaction = &core.Compaction{ID: "compact", StartedAt: at.Add(-time.Minute), Status: "submitted", Continuation: true}
	if err := l.Save(a); err != nil {
		t.Fatal(err)
	}
	e := core.Envelope{ID: "context-1", Token: "0123456789abcdef", From: core.Sender{Kind: core.SenderGangline, Name: "context-band"}, Recipient: a.ID, To: a.Name, Message: core.Message{Text: "spooled"}, CreatedAt: at}
	if err := f.run.publishOnce(l, &a, e); err != nil {
		t.Fatal(err)
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.run.confirmCompactionHook(a.ID, hookNotice{Kind: "compaction-finished", SessionID: "s", At: at.Add(time.Second)}); err != nil {
		t.Fatal(err)
	}
	if queued, err := p.ListNew(); err != nil || len(queued) != 0 {
		t.Fatalf("stale context note remains: %+v, %v", queued, err)
	}
	if got, err := p.Read(); err != nil || got.LastFailed != retained.ID {
		t.Fatalf("discard replaced the retained receipt: %q, %v", got.LastFailed, err)
	}
	if _, err := p.ReadEnvelope("failed", retained.ID); err != nil {
		t.Fatalf("discard removed the retained receipt: %v", err)
	}
}
