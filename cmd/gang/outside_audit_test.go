//go:build acceptance

package main

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/store"
)

func outsideAuditEvents(t *testing.T, f *stateFixture, kind string) []core.Event {
	t.Helper()
	data, err := os.ReadFile(f.run.team.Log)
	if err != nil {
		t.Fatal(err)
	}
	var events []core.Event
	if err := store.ReadLog(strings.NewReader(string(data)), func(e core.Event) error {
		if e.Type == kind {
			events = append(events, e)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return events
}

func TestOutsideMailAuditRetainsAcknowledgedEnvelope(t *testing.T) {
	f := newStateFixture(t)
	paths, principal := registerOutsideFixture(t, f)
	if err := f.cmd.execute([]string{"send", "toby", "--from", "operator", "review this report"}); err != nil {
		t.Fatal(err)
	}
	var envelope core.Envelope
	if err := paths.WithOutside("unit", principal, func(m *store.OutsideMailbox) error {
		pending := m.Pending()
		if len(pending) != 1 {
			t.Fatalf("pending = %+v", pending)
		}
		envelope = pending[0]
		if err := m.Ack([]core.EnvelopeID{envelope.ID}, f.cmd.now()); err != nil {
			return err
		}
		return m.Ack([]core.EnvelopeID{envelope.ID, "absent"}, f.cmd.now())
	}); err != nil {
		t.Fatal(err)
	}
	pending, err := paths.ReadOutsidePending("unit", principal)
	if err != nil || len(pending) != 0 {
		t.Fatalf("acknowledged mail = %+v, %v", pending, err)
	}
	queued := outsideAuditEvents(t, f, "send_queued")
	if len(queued) != 1 || queued[0].Envelope == nil || *queued[0].Envelope != envelope {
		t.Fatalf("queued audit = %+v", queued)
	}
	acks := outsideAuditEvents(t, f, "outside_acknowledged")
	if len(acks) != 1 || acks[0].ID != string(envelope.ID) || acks[0].HitchID != principal.ID {
		t.Fatalf("ack audit = %+v", acks)
	}
}

func TestOutsideMailAuditFailureReportsCompletedMutation(t *testing.T) {
	f := newStateFixture(t)
	paths, principal := registerOutsideFixture(t, f)
	if err := os.Rename(f.run.team.Log, f.run.team.Log+".saved"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(f.run.team.Log, 0700); err != nil {
		t.Fatal(err)
	}
	err := f.cmd.execute([]string{"send", "toby", "--from", "operator", "retained"})
	if err == nil || !strings.Contains(err.Error(), "queued for toby, but audit append failed") || !strings.Contains(err.Error(), "do not resend") {
		t.Fatalf("queue partial outcome = %v", err)
	}
	pending, err := paths.ReadOutsidePending("unit", principal)
	if err != nil || len(pending) != 1 {
		t.Fatalf("queued mail = %+v, %v", pending, err)
	}
	err = paths.WithOutside("unit", principal, func(m *store.OutsideMailbox) error {
		return m.Ack([]core.EnvelopeID{pending[0].ID}, f.cmd.now())
	})
	var pathErr *os.PathError
	if !errors.As(err, &pathErr) || !strings.Contains(err.Error(), "acknowledged, but audit append failed") {
		t.Fatalf("ack partial outcome = %v", err)
	}
	pending, err = paths.ReadOutsidePending("unit", principal)
	if err != nil || len(pending) != 0 {
		t.Fatalf("acknowledged mail = %+v, %v", pending, err)
	}
}

func TestOutsideInboundAuditFailureRetainsRetryableIntent(t *testing.T) {
	f := newStateFixture(t)
	paths, principal := registerOutsideFixture(t, f)
	f.add(t, "worker-id", "worker", "codex")
	if err := os.Rename(f.run.team.Log, f.run.team.Log+".saved"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(f.run.team.Log, 0700); err != nil {
		t.Fatal(err)
	}
	first, err := f.run.outsideSend(principal.String(), "worker", "approved", "request")
	if err == nil || !strings.Contains(err.Error(), "outcome queued, but audit append failed") {
		t.Fatalf("partial outcome = %+v, %v", first, err)
	}
	if err := paths.WithOutside("unit", principal, func(m *store.OutsideMailbox) error {
		var intent outsideSendIntent
		found, err := m.Get("send:request", &intent)
		if err == nil && (!found || intent.Envelope == nil || intent.Receipt != nil) {
			t.Fatalf("lost retryable intent = %+v", intent)
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(f.run.team.Log); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(f.run.team.Log+".saved", f.run.team.Log); err != nil {
		t.Fatal(err)
	}
	again, err := f.run.outsideSend(principal.String(), "worker", "approved", "request")
	if err != nil || again != first {
		t.Fatalf("retry = %+v, %v", again, err)
	}
	p, err := f.run.team.Agent("worker-id")
	if err != nil {
		t.Fatal(err)
	}
	pending, err := p.ListNew()
	if err != nil || len(pending) != 1 {
		t.Fatalf("retry publication = %+v, %v", pending, err)
	}
	if events := outsideAuditEvents(t, f, "send_queued"); len(events) != 1 || events[0].Envelope == nil || events[0].Envelope.ID != pending[0].ID {
		t.Fatalf("repaired audit = %+v", events)
	}
}

func TestOutsideInboundAuditSurvivesReceiptSettlement(t *testing.T) {
	f := newStateFixture(t)
	_, principal := registerOutsideFixture(t, f)
	f.add(t, "worker-id", "worker", "codex")
	first, err := f.run.outsideSend(principal.String(), "worker", "approved", "request")
	if err != nil {
		t.Fatal(err)
	}
	again, err := f.run.outsideSend(principal.String(), "worker", "approved", "request")
	if err != nil || again != first {
		t.Fatalf("replay = %+v, %v", again, err)
	}
	events := outsideAuditEvents(t, f, "send_queued")
	if len(events) != 1 || events[0].Envelope == nil {
		t.Fatalf("send audit = %+v", events)
	}
	e := events[0].Envelope
	if string(e.ID) != first.ID || e.From != principal.Sender() || e.To != "worker" || e.Message.Text != "approved" || e.CreatedAt.IsZero() {
		t.Fatalf("incomplete send audit = %+v", e)
	}
}
