//go:build acceptance

package main

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/store"
)

// A failed receipt keeps the prompt that failed to match, after a later turn
// replaces the witness; a receipt that later verifies drops it.
func TestMismatchedPromptIsRetainedUntilVerified(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "codex")
	p, _ := f.run.team.Agent(a.ID)
	f.input.submit = func(string) error {
		return p.WriteWitness(store.Witness{ID: "wrong", At: f.cmd.now(), Prompt: "rewritten\tprompt"})
	}
	cmd := f.cmd
	cmd.stdin = strings.NewReader("hello")
	cmd.newWatch = func(string) (changeWait, error) {
		return waitFixture{func(context.Context) error { return errors.New("test boundary: no matching receipt") }}, nil
	}
	var ce commandError
	if err := cmd.send([]string{"worker", "--from", "operator"}); !errors.As(err, &ce) || ce.status != exitUnknown {
		t.Fatalf("mismatch result: %v", err)
	}
	a, err := p.Read()
	if err != nil {
		t.Fatal(err)
	}
	e, err := p.ReadEnvelope("failed", a.LastFailed)
	if err != nil {
		t.Fatal(err)
	}
	if e.Outcome != "unverified" || e.Witnessed != "rewritten\tprompt" {
		t.Fatalf("failed receipt: outcome %q, witnessed %q", e.Outcome, e.Witnessed)
	}
	if err := p.WriteWitness(store.Witness{ID: "later", At: f.cmd.now(), Prompt: "a later turn"}); err != nil {
		t.Fatal(err)
	}
	if e, err = p.ReadEnvelope("failed", a.LastFailed); err != nil || e.Witnessed != "rewritten\tprompt" {
		t.Fatalf("after a later turn: witnessed %q, err %v", e.Witnessed, err)
	}
	wire, err := envelopeText(e)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.WriteWitness(store.Witness{ID: "late", At: f.cmd.now(), Prompt: wire}); err != nil {
		t.Fatal(err)
	}
	l, _, err := f.run.acquire(a.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	l.Close()
	got, err := p.ReadEnvelope("cur", e.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Outcome != "delivered" || got.Witnessed != "" {
		t.Fatalf("verified receipt: outcome %q, witnessed %q", got.Outcome, got.Witnessed)
	}
}

func TestStartupRecoveryRetainsMismatchedPrompt(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "codex")
	lead := f.add(t, "b", "lead", "codex")
	p, _ := f.run.team.Agent(a.ID)
	e := core.Envelope{ID: "original", Token: "0123456789abcdef", Recipient: a.ID, To: a.Name, From: agentSender(lead), Purpose: "assignment", Message: core.Message{Text: "Assignment: fix it."}, CreatedAt: f.cmd.now()}
	if err := p.Publish(e); err != nil {
		t.Fatal(err)
	}
	l, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	a.Input = &core.InputIntent{ID: string(e.ID), Kind: "envelope", At: f.cmd.now()}
	if err := f.run.finishInput(l, &a, e, "unverified", "another surface owns input"); err != nil {
		t.Fatal(err)
	}
	l.Close()
	wire, err := envelopeText(e)
	if err != nil {
		t.Fatal(err)
	}
	f.input.pasted = wire
	f.input.screen = screenWithText("› " + wire)
	f.input.submit = func(string) error {
		return p.WriteWitness(store.Witness{ID: "recovered", At: f.cmd.now(), Prompt: "rewritten startup", SessionID: "s"})
	}
	f.cmd.inputBackend = submitOnlyFixture{f.input}
	var ce commandError
	if err := f.cmd.hitch([]string{"worker", "--recover"}); !errors.As(err, &ce) || ce.status != exitUnknown {
		t.Fatalf("mismatched recovery: %v", err)
	}
	got, err := p.ReadEnvelope("failed", e.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Outcome != "unverified" || got.Witnessed != "rewritten startup" {
		t.Fatalf("failed startup: outcome %q, witnessed %q", got.Outcome, got.Witnessed)
	}
}
