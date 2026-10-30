package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/store"
)

func registerOutsideFixture(t *testing.T, f *stateFixture) (store.Paths, store.OutsidePrincipal) {
	t.Helper()
	credential := filepath.Join(t.TempDir(), "outside-credential")
	if err := f.cmd.execute([]string{"outside", "register", "toby", "--credential-file", credential}); err != nil {
		t.Fatal(err)
	}
	token, err := os.ReadFile(credential)
	if err != nil {
		t.Fatal(err)
	}
	paths := store.Paths{Root: f.env["GANG_STATE_ROOT"]}
	principal, err := paths.AuthenticateOutside("unit", strings.TrimSpace(string(token)))
	if err != nil {
		t.Fatal(err)
	}
	f.out.Reset()
	return paths, principal
}
func TestOutsideCLIRegistrationSendRosterDrop(t *testing.T) {
	f := newStateFixture(t)
	paths, principal := registerOutsideFixture(t, f)
	if err := f.cmd.execute([]string{"send", "toby", "--from", "operator", "review this report"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(f.out.String(), "\tqueued\n") {
		t.Fatalf("missing queued receipt %q", f.out.String())
	}
	if f.input.submits != 0 {
		t.Fatal("mailbox send touched native input")
	}
	if err := paths.WithOutside("unit", principal, func(m *store.OutsideMailbox) error {
		pending := m.Pending()
		if len(pending) != 1 || pending[0].Message.Text != "review this report" || pending[0].From.Kind != core.SenderSelfDeclared {
			t.Fatalf("pending %+v", pending)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	f.out.Reset()
	if err := f.cmd.execute([]string{"roster", "--json"}); err != nil {
		t.Fatal(err)
	}
	var roster rosterJSON
	if err := json.Unmarshal([]byte(f.out.String()), &roster); err != nil {
		t.Fatal(err)
	}
	if len(roster.Agents) != 1 || roster.Agents[0].Kind != "outside" || roster.Agents[0].Name != "toby" {
		t.Fatalf("roster %+v", roster)
	}
	if err := f.cmd.execute([]string{"drop", "toby"}); err != nil {
		t.Fatal(err)
	}
	if _, err := paths.ResolveOutsidePrincipal("unit", principal.String()); !errors.Is(err, store.ErrOutsideUnauthorized) {
		t.Fatalf("drop did not revoke: %v", err)
	}
}
func TestOutsideCLIFlagsDoNotConferVerifiedIdentity(t *testing.T) {
	f := newStateFixture(t)
	paths, principal := registerOutsideFixture(t, f)
	f.add(t, "worker-id", "worker", "codex")
	// A declared outside name remains unverified even while it is registered.
	p, _ := f.run.team.Agent("worker-id")
	lock, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	if err := f.cmd.send([]string{"worker", "--from", "toby", "forged"}); err != nil {
		_ = lock.Close()
		t.Fatal(err)
	}
	if err := lock.Close(); err != nil {
		t.Fatal(err)
	}
	pending, err := p.ListNew()
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 || pending[0].From.Kind != core.SenderSelfDeclared {
		t.Fatalf("declaration verified %+v", pending)
	}
	f.env["GANG_AGENT_ID"] = string(principal.ID)
	f.env["GANG_OUTSIDE_PRINCIPAL"] = principal.String()
	if err := f.cmd.send([]string{"worker", "forged by env"}); err == nil {
		t.Fatal("outside registration used as native identity")
	}
	if _, err := paths.ResolveOutsidePrincipal("unit", principal.String()); err != nil {
		t.Fatal(err)
	}
}
func TestOutsideInboundSendIdentityAndIdempotency(t *testing.T) {
	f := newStateFixture(t)
	paths, principal := registerOutsideFixture(t, f)
	f.add(t, "worker-id", "worker", "codex")
	var ticks int
	f.run.cmd.detach = func(id string, _ hookNotice) error {
		if id != "worker-id" {
			t.Fatalf("tick %s", id)
		}
		ticks++
		return nil
	}
	receipt, err := f.run.outsideSend(principal.String(), "worker", "approved", "request-one")
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Outcome != "queued" || receipt.ID == "" || ticks != 1 {
		t.Fatalf("receipt %+v ticks %d", receipt, ticks)
	}
	p, _ := f.run.team.Agent("worker-id")
	pending, err := p.ListNew()
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 || pending[0].From != principal.Sender() {
		t.Fatalf("verified inbound %+v", pending)
	}
	wire, err := envelopeText(pending[0])
	if err != nil || !strings.Contains(wire, "[gang:toby#") || strings.Contains(wire, "self-declared") {
		t.Fatalf("wire %q %v", wire, err)
	}
	repeat, err := f.run.outsideSend(principal.String(), "worker", "approved", "request-one")
	if err != nil || repeat != receipt || ticks != 1 {
		t.Fatalf("duplicate receipt %+v ticks %d err %v", repeat, ticks, err)
	}
	if _, err := f.run.outsideSend(principal.String(), "worker", "different", "request-one"); err == nil {
		t.Fatal("key reused for different body")
	}
	// Simulate loss of the mailbox receipt after native publication. Retry finds
	// the same persisted intent and native envelope, then repairs its receipt.
	if err := paths.WithOutside("unit", principal, func(m *store.OutsideMailbox) error {
		var intent outsideSendIntent
		if _, err := m.Get("send:request-one", &intent); err != nil {
			return err
		}
		intent.Receipt = nil
		intent.Envelope = &pending[0]
		intent.Attempted = true
		return m.Put("send:request-one", intent)
	}); err != nil {
		t.Fatal(err)
	}
	repeat, err = f.run.outsideSend(principal.String(), "worker", "approved", "request-one")
	if err != nil || repeat != receipt {
		t.Fatalf("lost receipt retry %+v %v", repeat, err)
	}
	pending, err = p.ListNew()
	if err != nil || len(pending) != 1 {
		t.Fatalf("duplicate published %+v %v", pending, err)
	}
	f.env["GANGLINE_HITCH_ID"] = "worker-id"
	if err := f.cmd.tick([]string{"--agent", "worker"}); err != nil {
		t.Fatal(err)
	}
	delivered, err := p.ReadEnvelope("cur", core.EnvelopeID(receipt.ID))
	if err != nil || delivered.From != principal.Sender() || f.input.submits != 1 {
		t.Fatalf("native consumer delivery %+v submits %d err %v", delivered, f.input.submits, err)
	}
	if !strings.HasPrefix(f.input.pasted, "[gang:toby#"+delivered.Token+"] approved") {
		t.Fatalf("native envelope %q", f.input.pasted)
	}
	mailboxState, err := os.ReadFile(filepath.Join(f.run.team.Directory, "outside", string(principal.ID)+".json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(mailboxState), "approved") || strings.Contains(string(mailboxState), "request-one") || strings.Contains(string(mailboxState), "receipt") || strings.Contains(string(mailboxState), "fingerprint") {
		t.Fatalf("mailbox snapshot retained settled history: %s", mailboxState)
	}
	if err := paths.WithOutside("unit", principal, func(m *store.OutsideMailbox) error {
		var settled outsideSendIntent
		found, err := m.Get("send:request-one", &settled)
		if err != nil {
			return err
		}
		if !found || settled.Envelope != nil || settled.Attempted || settled.Receipt == nil || *settled.Receipt != receipt || settled.Fingerprint != outsideSendFingerprint("worker", "approved") {
			t.Fatalf("settled record contains pending payload: %+v", settled)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if replay, err := f.run.outsideSend(principal.String(), "worker", "approved", "request-one"); err != nil || replay != receipt {
		t.Fatalf("settled replay %+v %v", replay, err)
	}
	if _, err := f.run.outsideSend(principal.String(), "worker", "altered after delivery", "request-one"); err == nil {
		t.Fatal("settled key accepted changed body")
	}
	if err := paths.RotateOutside("unit", "toby", filepath.Join(t.TempDir(), "rotated"), credentialTestOrigin()); err != nil {
		t.Fatal(err)
	}
	if _, err := f.run.outsideSend(principal.String(), "worker", "approved", "request-one"); !errors.Is(err, store.ErrOutsideUnauthorized) {
		t.Fatalf("revoked sender replay: %v", err)
	}
}
func TestOutsideInboundUncertainPublicationDoesNotDuplicate(t *testing.T) {
	f := newStateFixture(t)
	paths, principal := registerOutsideFixture(t, f)
	f.add(t, "worker-id", "worker", "codex")
	receipt, err := f.run.outsideSend(principal.String(), "worker", "approved", "uncertain")
	if err != nil {
		t.Fatal(err)
	}
	p, _ := f.run.team.Agent("worker-id")
	envelope, err := p.ReadEnvelope("new", core.EnvelopeID(receipt.ID))
	if err != nil {
		t.Fatal(err)
	}
	envelopePath, _ := p.EnvelopePath("new", core.EnvelopeID(receipt.ID))
	if err := os.Remove(envelopePath); err != nil {
		t.Fatal(err)
	}
	if err := paths.WithOutside("unit", principal, func(m *store.OutsideMailbox) error {
		var intent outsideSendIntent
		if _, err := m.Get("send:uncertain", &intent); err != nil {
			return err
		}
		intent.Receipt = nil
		intent.Envelope = &envelope
		intent.Attempted = true
		return m.Put("send:uncertain", intent)
	}); err != nil {
		t.Fatal(err)
	}
	retry, err := f.run.outsideSend(principal.String(), "worker", "approved", "uncertain")
	if err != nil || retry.Outcome != "unknown" || retry.ID != receipt.ID {
		t.Fatalf("uncertain receipt %+v %v", retry, err)
	}
	pending, err := p.ListNew()
	if err != nil || len(pending) != 0 {
		t.Fatalf("uncertain replay published %+v %v", pending, err)
	}
}
