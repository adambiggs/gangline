//go:build acceptance

package main

import (
	"context"
	"errors"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/substrate"
)

func deliveryContext(a core.Agent, c harness.Collar, b harnessInput) core.Reading {
	r, _ := deliveryReadings(a, c, b)
	return r
}

// stampReading observes used thousandths of a one-million-token window.
func stampReading(at time.Time, used int64) core.Reading {
	limit, percent := int64(1_000_000), float64(used)/10
	used *= 1000
	return core.Reading{Kind: "context", Source: "fixture", Status: "observed", Model: "gpt-test", At: &at, Used: &used, Limit: &limit, Percent: &percent}
}

func stampMail(a core.Agent, id string, at time.Time) core.Envelope {
	return core.Envelope{ID: core.EnvelopeID(id), Token: "0123456789abcdef", From: core.Sender{Kind: core.SenderPeer, Host: "peer", Name: "lead"}, Recipient: a.ID, To: a.Name, Message: core.Message{Text: "work"}, CreatedAt: at}
}

func TestContextStampRecipientDeliveryAndOnceOnlyCrossings(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "codex")
	f.env["GANGLINE_HITCH_ID"] = "a"
	p, _ := f.run.team.Agent(a.ID)
	l, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	a.Native.Context = stampReading(f.cmd.now(), 950)
	c, err := harness.EmbeddedCollar("codex")
	if err != nil {
		t.Fatal(err)
	}
	if err := f.run.noteContextBands(&a, c); err != nil {
		t.Fatal(err)
	}
	if err := f.run.publishContextNotes(l, &a); err != nil {
		t.Fatal(err)
	}
	queued, err := p.ListNew()
	if err != nil || len(queued) != 0 {
		t.Fatalf("no immediate standalone: %+v %v", queued, err)
	}
	e := stampMail(a, "peer-work", f.cmd.now())
	if err := p.Publish(e); err != nil {
		t.Fatal(err)
	}
	outcome, _, err := f.run.deliver(l, &a, e, f.input, c, "")
	if err != nil || outcome != "delivered" {
		t.Fatalf("deliver %s: %v", outcome, err)
	}
	got, err := p.ReadEnvelope("cur", e.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ContextStamp == nil || len(got.ContextStamp.Crossings) != 2 || !strings.Contains(f.input.pasted, "[/gang:peer:lead#0123456789abcdef]\n[gang ctx=950k/1M at=09-22T10:00:00Z; yellow 75% crossed: save state and compact/clear; red 90% crossed") || !strings.HasSuffix(f.input.pasted, " /]") || !strings.HasPrefix(f.input.pasted, "[gang:peer:lead#") {
		t.Fatalf("recipient stamp: %s %+v", f.input.pasted, got)
	}
	if len(a.ContextBands.Pending) != 0 {
		t.Fatalf("delivered crossing remains: %+v", a.ContextBands)
	}
	// The normal grace interval cannot publish a duplicate fallback afterward.
	now := f.cmd.now().Add(watchdogTimeout)
	f.run.cmd.clock = func() time.Time { return now }
	if err := f.run.publishContextNotes(l, &a); err != nil {
		t.Fatal(err)
	}
	queued, err = p.ListNew()
	if err != nil || len(queued) != 0 {
		t.Fatalf("duplicate fallback %+v %v", queued, err)
	}
}

func TestContextStampFallbackWaitsOneTickAndRespectsComposer(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "codex")
	f.env["GANGLINE_HITCH_ID"] = "a"
	p, _ := f.run.team.Agent(a.ID)
	l, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	a.Native.Context = stampReading(f.cmd.now(), 800)
	c, _ := harness.EmbeddedCollar("codex")
	if err := f.run.noteContextBands(&a, c); err != nil {
		t.Fatal(err)
	}
	if err := l.Save(a); err != nil {
		t.Fatal(err)
	}
	now := f.cmd.now().Add(watchdogTimeout - time.Nanosecond)
	f.run.cmd.clock = func() time.Time { return now }
	if err := f.run.publishContextNotes(l, &a); err != nil {
		t.Fatal(err)
	}
	q, _ := p.ListNew()
	if len(q) != 0 {
		t.Fatal("fallback before one tick")
	}
	now = now.Add(time.Nanosecond)
	if err := f.run.publishContextNotes(l, &a); err != nil {
		t.Fatal(err)
	}
	q, err = p.ListNew()
	if err != nil || len(q) != 1 {
		t.Fatalf("due fallback %+v %v", q, err)
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	f.input.screen = screenWithText("READY", "› draft")
	if _, err := f.run.drain(a.ID, ""); err != nil {
		t.Fatal(err)
	}
	if f.input.submits != 0 {
		t.Fatal("draft overwritten")
	}
	f.input.screen = screenWithText("READY", "› ")
	if _, err := f.run.drain(a.ID, ""); err != nil {
		t.Fatal(err)
	}
	if f.input.submits != 1 || !regexp.MustCompile(`^\[gang#[0-9a-f]{16}\] \[/gang#[0-9a-f]{16}\]\n\[gang ctx=800k/1M at=\d\d-\d\dT\d\d:\d\d:\d\dZ; yellow 75% crossed[^\n]* /\]$`).MatchString(f.input.pasted) || strings.Contains(f.input.pasted, "Context band") {
		t.Fatalf("stamp-only fallback: %s (%d)", f.input.pasted, f.input.submits)
	}
	if _, err := f.run.drain(a.ID, ""); err != nil {
		t.Fatal(err)
	}
	if f.input.submits != 1 {
		t.Fatal("fallback repeated")
	}
}

func TestContextStampUnknownStartupAndRetainedResume(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "codex")
	p, _ := f.run.team.Agent(a.ID)
	l, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	c, _ := harness.EmbeddedCollar("codex")
	e := stampMail(a, "startup", f.cmd.now())
	e.Purpose = "assignment"
	e.Startup = &core.StartupSections{Contract: "contract", Doctrine: "doctrine", Role: "role"}
	e, err = f.run.stampEnvelope(l, &a, e, f.input, c)
	if err != nil {
		t.Fatal(err)
	}
	wire, err := envelopeText(e)
	if err != nil || strings.Count(wire, "ctx=unknown") != 1 || !strings.HasSuffix(wire, "[/gang:peer:lead#0123456789abcdef]\n[gang ctx=unknown at=09-22T10:00:00Z /]") {
		t.Fatalf("startup stamps %s %v", wire, err)
	}
	e.ContextStamp = &core.ContextStamp{Text: "ctx=800k/1M at=09-22T10:00:00Z; yellow crossed: save state and compact/clear"}
	wire, err = envelopeText(e)
	if err != nil || strings.Count(wire, "ctx=800k/1M") != 1 || strings.Count(wire, "yellow crossed") != 1 || !strings.HasSuffix(wire, "\n[gang ctx=800k/1M at=09-22T10:00:00Z; yellow crossed: save state and compact/clear /]") {
		t.Fatalf("startup crossing repeated: %s %v", wire, err)
	}
	a.Compaction = &core.Compaction{ID: "c", Resume: core.Message{Text: "resume"}, ResumeToken: "aaaaaaaaaaaaaaaa", ResumeFrom: core.Sender{Kind: core.SenderGangline, Name: "compact"}}
	resume := core.Envelope{ID: "resume-c", Token: a.Compaction.ResumeToken, From: a.Compaction.ResumeFrom, Message: a.Compaction.Resume, Purpose: "resume", ContextStamp: e.ContextStamp}
	if err := p.Publish(resume); err != nil {
		t.Fatal(err)
	}
	retained, err := f.run.retainedResume(a)
	if err != nil || retained.ContextStamp == nil || retained.ContextStamp.Text != e.ContextStamp.Text {
		t.Fatalf("resume not retained: %+v %v", retained, err)
	}
	// Transcript errors have the same unknown result as context, not stale data.
	a.Native.Context = stampReading(f.cmd.now(), 950)
	a.Native.SessionID = "s"
	a.Native.Transcript = "/missing-context-fixture"
	if r := deliveryContext(a, c, f.input); contextStampText(r) != "ctx=unknown" {
		t.Fatalf("stale reading exposed: %+v", r)
	}
}

func TestContextStampResetKeepsMailAndToleratesRetiredEnvelopeFields(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "codex")
	p, _ := f.run.team.Agent(a.ID)
	l, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	c, _ := harness.EmbeddedCollar("codex")
	a.Native.Context = stampReading(f.cmd.now(), 800)
	e := stampMail(a, "ordinary", f.cmd.now())
	e, err = f.run.stampEnvelope(l, &a, e, f.input, c)
	if err != nil {
		t.Fatal(err)
	}
	at := f.cmd.now().Add(time.Second)
	if err := f.run.acceptContextReadings(&a, c, []core.Reading{{Kind: "compaction-finished", At: &at}}); err != nil {
		t.Fatal(err)
	}
	if err := f.run.publishContextNotes(l, &a); err != nil {
		t.Fatal(err)
	}
	got, err := p.ReadEnvelope("new", e.ID)
	if err != nil || got.ContextStamp != nil || got.Message != e.Message {
		t.Fatalf("reset damaged mail: %+v %v", got, err)
	}
	if len(a.ContextBands.Pending) != 0 {
		t.Fatal("reset retained crossing")
	}
	path, err := p.EnvelopePath("new", e.ID)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data = append([]byte(`{"retired_context_notice":true,`), data[1:]...)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := p.ReadEnvelope("new", e.ID); err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
}

func TestContextStampDirectCompactionContinuation(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "codex")
	a.Native.Context = stampReading(f.cmd.now(), 800)
	a.Compaction = &core.Compaction{ID: "c", Status: "submitted", StartedAt: f.cmd.now(),
		Resume: core.Message{Text: "Continue from saved state."}, ResumeFrom: core.Sender{Kind: core.SenderGangline, Name: "compact"}}
	f.input.screen = screenWithText("◦ Compacting context (0s • esc to interrupt)", "› ")
	stopped := errors.New("fixture stopped after paste")
	f.run.cmd.settleInput = func(context.Context, harnessInput, substrate.PaneID, harness.Collar, time.Duration) error {
		return stopped
	}
	p, _ := f.run.team.Agent(a.ID)
	l, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	c, _ := harness.EmbeddedCollar("codex")
	if err := f.run.queueCompactionResume(l, &a, f.input, c, "/compact"); !errors.Is(err, stopped) {
		t.Fatal(err)
	}
	e, err := p.ReadEnvelope("failed", "resume-c")
	if err != nil || e.ContextStamp == nil || !strings.Contains(f.input.pasted, "\n[gang ctx=800k/1M at=09-22T10:00:00Z; yellow 75% crossed") {
		t.Fatalf("unstamped continuation: %+v %s %v", e, f.input.pasted, err)
	}
	wire, err := envelopeText(e)
	if err != nil || wire != f.input.pasted {
		t.Fatalf("continuation receipt changed: %s %v", wire, err)
	}
}

func TestContextStampUsesVerifiedIncrementalTranscript(t *testing.T) {
	f := contextFixtureWithoutCachedState(t, parentContextRecords, func(*core.Agent) {})
	p, _ := f.run.team.Agent("a")
	l, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	a, err := p.Read()
	if err != nil {
		t.Fatal(err)
	}
	c, _ := harness.EmbeddedCollar("codex")
	if err := f.run.refreshNative(l, &a, c); err != nil {
		t.Fatal(err)
	}
	if a.Native.Offset == 0 {
		t.Fatal("fixture has no retained cursor")
	}
	if got := deliveryContext(a, c, f.input); got.Used == nil || *got.Used != 150 || *got.Limit != 1000 {
		t.Fatalf("%+v", got)
	}
	file, err := os.OpenFile(a.Native.Transcript, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, err = file.WriteString(`{"timestamp":"2026-09-22T10:00:02Z","type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"total_tokens":300},"model_context_window":1000}}}` + "\n")
	closeErr := file.Close()
	if err != nil || closeErr != nil {
		t.Fatalf("append %v %v", err, closeErr)
	}
	if got := deliveryContext(a, c, f.input); got.Used == nil || *got.Used != 300 || *got.Limit != 1000 {
		t.Fatalf("%+v", got)
	}
}

func TestContextStampUnverifiableCachedRecordIsUnknown(t *testing.T) {
	f := contextFixtureWithoutCachedState(t, parentContextRecords, func(*core.Agent) {})
	p, _ := f.run.team.Agent("a")
	l, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	a, err := p.Read()
	if err != nil {
		t.Fatal(err)
	}
	c, _ := harness.EmbeddedCollar("codex")
	if err := f.run.refreshNative(l, &a, c); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(a.Native.Transcript)
	if err != nil {
		t.Fatal(err)
	}
	data = []byte(strings.ReplaceAll(string(data), `"type":"token_count"`, `"type":"other_event"`))
	if err := os.WriteFile(a.Native.Transcript, data, 0600); err != nil {
		t.Fatal(err)
	}
	if got := contextStampText(deliveryContext(a, c, f.input)); got != "ctx=unknown" {
		t.Fatal(got)
	}
}

func TestContextStampBatchPreservesEachCrossingAndCustomBands(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "codex")
	a.ContextBandThresholds = &core.ContextBandThresholds{Early: .2, Late: .4}
	c, _ := harness.EmbeddedCollar("codex")
	c.ContextBands = map[string][]harness.ContextBand{"*": {{Name: "early", At: .75, Message: "{{band}} {{threshold_percent}}%: save state and compact/clear"}, {Name: "late", At: .90}}}
	for i, used := range []int64{200, 400, 100, 400} {
		if err := f.run.acceptContextReadings(&a, c, []core.Reading{stampReading(f.cmd.now().Add(time.Duration(i)*time.Second), used)}); err != nil {
			t.Fatal(err)
		}
	}
	if len(a.ContextBands.Pending) != 4 {
		t.Fatalf("batch lost crossings: %+v", a.ContextBands)
	}
	p, _ := f.run.team.Agent(a.ID)
	l, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	e, err := f.run.stampEnvelope(l, &a, stampMail(a, "batch", f.cmd.now()), f.input, c)
	if err != nil || e.ContextStamp == nil || len(e.ContextStamp.Crossings) != 4 || strings.Count(e.ContextStamp.Text, "early 20%: save state and compact/clear") != 2 || strings.Count(e.ContextStamp.Text, "late 40% crossed") != 2 {
		t.Fatalf("batch stamp %+v %v", e, err)
	}
}

// The largest body a send accepts still fits once stamped: band actions that
// would not fit wait for the next envelope.
func TestContextStampKeepsAcceptedMessageWithinBudget(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "codex")
	c, _ := harness.EmbeddedCollar("codex")
	for i, used := range []int64{800, 950} {
		if err := f.run.acceptContextReadings(&a, c, []core.Reading{stampReading(f.cmd.now().Add(time.Duration(i)*time.Second), used)}); err != nil {
			t.Fatal(err)
		}
	}
	if len(a.ContextBands.Pending) != 2 {
		t.Fatalf("crossings = %+v", a.ContextBands)
	}
	full := stampMail(a, "full", f.cmd.now())
	full.Crossed = "0123456789abcdef,fedcba9876543210"
	low, high := 0, maximumMessageBytes
	for low < high {
		mid := (low + high + 1) / 2
		full.Message.Text = strings.Repeat("x", mid)
		if _, err := queuedEnvelopeText(full); err == nil {
			low = mid
		} else {
			high = mid - 1
		}
	}
	full.Message.Text = strings.Repeat("x", low)
	if _, err := envelopeText(full); err != nil {
		t.Fatal(err)
	}
	full.Message.Text += "x"
	if _, err := queuedEnvelopeText(full); err == nil {
		t.Fatal("a body with no room for its stamp was accepted")
	}
	full.Message.Text = full.Message.Text[1:]
	p, _ := f.run.team.Agent(a.ID)
	l, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	e, err := f.run.stampEnvelope(l, &a, full, f.input, c)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := envelopeText(e); err != nil || len(e.ContextStamp.Crossings) != 0 || !strings.Contains(e.ContextStamp.Text, "crossed=0123456789abcdef,fedcba9876543210") {
		t.Fatalf("stamped %+v: %v", e.ContextStamp, err)
	}
	if err := f.run.settleContextStamp(l, &a, e, "delivered"); err != nil {
		t.Fatal(err)
	}
	next, err := f.run.stampEnvelope(l, &a, stampMail(a, "next", f.cmd.now()), f.input, c)
	if err != nil || next.ContextStamp == nil || len(next.ContextStamp.Crossings) != 2 {
		t.Fatalf("waiting crossings not carried: %+v %v", next.ContextStamp, err)
	}
}

func TestContextStampCrossingRepeatsUntilVerifiedSubmission(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "codex")
	a.Native.Context = stampReading(f.cmd.now(), 800)
	p, _ := f.run.team.Agent(a.ID)
	l, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	c, _ := harness.EmbeddedCollar("codex")
	for i, id := range []string{"uncertain", "verified", "after"} {
		e, err := f.run.stampEnvelope(l, &a, stampMail(a, id, f.cmd.now()), f.input, c)
		if err != nil {
			t.Fatal(err)
		}
		crossed := strings.Contains(e.ContextStamp.Text, "yellow 75% crossed")
		if crossed != (i < 2) {
			t.Fatalf("carrier %s stamp %+v", id, e.ContextStamp)
		}
		outcome := "unverified"
		if i == 1 {
			outcome = "delivered"
		}
		if err := f.run.settleContextStamp(l, &a, e, outcome); err != nil {
			t.Fatal(err)
		}
	}
}

func TestContextStampTranscriptResetDiscardsOldCrossing(t *testing.T) {
	f := contextFixtureWithoutCachedState(t, parentContextRecords, func(*core.Agent) {})
	p, _ := f.run.team.Agent("a")
	l, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	a, err := p.Read()
	if err != nil {
		t.Fatal(err)
	}
	c, _ := harness.EmbeddedCollar("codex")
	if err := f.run.refreshNative(l, &a, c); err != nil {
		t.Fatal(err)
	}
	a.ContextBands.Pending = []core.ContextBandNote{{Band: "yellow", Reading: stampReading(f.cmd.now(), 800), Envelope: core.Envelope{ID: "old-crossing"}}}
	file, err := os.OpenFile(a.Native.Transcript, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, err = file.WriteString(`{"timestamp":"2026-09-22T10:00:02Z","type":"event_msg","payload":{"type":"context_compacted"}}` + "\n")
	closeErr := file.Close()
	if err != nil || closeErr != nil {
		t.Fatalf("append %v %v", err, closeErr)
	}
	e, err := f.run.stampEnvelope(l, &a, stampMail(a, "reset-tail", f.cmd.now()), f.input, c)
	if err != nil || e.ContextStamp == nil || len(e.ContextStamp.Crossings) > 0 || e.ContextStamp.Text != "ctx=unknown at=09-22T10:00:00Z" {
		t.Fatalf("stale reset action %+v %v", e, err)
	}
}

func TestContextStampNewBandFallbackPassesUncertainEarlierBand(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "codex")
	p, _ := f.run.team.Agent(a.ID)
	l, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	c, _ := harness.EmbeddedCollar("codex")
	a.Native.Context = stampReading(f.cmd.now(), 800)
	if err := f.run.noteContextBands(&a, c); err != nil {
		t.Fatal(err)
	}
	first := a.ContextBands.Pending[0].Envelope
	if err := p.Publish(first); err != nil {
		t.Fatal(err)
	}
	if err := l.Settle(&a, first, "unverified", "not confirmed"); err != nil {
		t.Fatal(err)
	}
	a.Native.Context = stampReading(f.cmd.now(), 950)
	if err := f.run.noteContextBands(&a, c); err != nil {
		t.Fatal(err)
	}
	now := f.cmd.now().Add(watchdogTimeout)
	f.run.cmd.clock = func() time.Time { return now }
	if err := f.run.publishContextNotes(l, &a); err != nil {
		t.Fatal(err)
	}
	queued, err := p.ListNew()
	if err != nil || len(queued) != 1 || queued[0].ID != a.ContextBands.Pending[1].Envelope.ID {
		t.Fatalf("new band starved %+v %v", queued, err)
	}
}

func TestContextStampKeepsEvaluatedThresholdAcrossCollarChanges(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "claude")
	p, _ := f.run.team.Agent(a.ID)
	l, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	c, err := harness.EmbeddedCollar("claude")
	if err != nil {
		t.Fatal(err)
	}
	used, limit := int64(200822), int64(1000000)
	percent := float64(used) / float64(limit) * 100
	at := f.cmd.now()
	a.Native.Context = core.Reading{Kind: "context", Source: "status-line", Status: "observed", Model: "claude-test", At: &at, Used: &used, Limit: &limit, Percent: &percent}
	if err := f.run.noteContextBands(&a, c); err != nil {
		t.Fatal(err)
	}
	if len(a.ContextBands.Pending) != 1 || a.ContextBands.Pending[0].Threshold != .20 {
		t.Fatalf("evaluated threshold not retained: %+v", a.ContextBands)
	}
	// Persist and reload the crossing before another process loads an overlay.
	if err := l.Save(a); err != nil {
		t.Fatal(err)
	}
	a, err = p.Read()
	if err != nil {
		t.Fatal(err)
	}
	c.ContextBands["*"] = []harness.ContextBand{{Name: "early", At: .75}, {Name: "late", At: .90}}
	e, err := f.run.stampEnvelope(l, &a, stampMail(a, "changed-collar", at), f.input, c)
	if err != nil {
		t.Fatal(err)
	}
	want := "ctx=200822/1000000; early 20% crossed: save state and compact/clear"
	if e.ContextStamp == nil || e.ContextStamp.Text != want {
		t.Fatalf("crossing relabeled by delivery collar: %+v, want %q", e.ContextStamp, want)
	}
}
