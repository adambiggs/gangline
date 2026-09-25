package main

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/store"
	"github.com/adambiggs/gangline/substrate"
)

type clearingCodexInput struct {
	*inputFixture
	clears, repastes int
	clearFails       bool
	pasteFails       bool
	promptAfterClear bool
	tailAfterClear   bool
	captures         int
	onCapture        func(int)
}

func (b *clearingCodexInput) Capture(ctx context.Context, pane substrate.PaneID) (substrate.Screen, error) {
	b.captures++
	if b.onCapture != nil {
		b.onCapture(b.captures)
	}
	return b.inputFixture.Capture(ctx, pane)
}

func (b *clearingCodexInput) SendKeys(ctx context.Context, pane substrate.PaneID, keys substrate.Keys) error {
	if len(keys.Names) == 1 && keys.Names[0] == "C-u" {
		b.clears++
		if !b.clearFails {
			b.pasted = ""
			b.screen = screenWithText("› ")
			if b.promptAfterClear {
				b.screen = screenWithText("Hooks need review", "› 1. Review hooks", "Press enter to confirm")
			}
			if b.tailAfterClear {
				b.screen = screenWithText("› ", "operator draft")
			}
		}
		return nil
	}
	if keys.Text != "" {
		b.repastes++
		if b.pasteFails {
			return fmt.Errorf("paste unavailable")
		}
		if err := b.inputFixture.SendKeys(ctx, pane, keys); err != nil {
			return err
		}
		b.screen = screenWithText(fmt.Sprintf("› [Pasted Content %d chars]", utf8.RuneCountInString(b.pasted)))
		return nil
	}
	return b.inputFixture.SendKeys(ctx, pane, keys)
}

func newCollapsedRecoveryFixture(t *testing.T) (*stateFixture, store.AgentPaths, core.Envelope, *clearingCodexInput, string) {
	t.Helper()
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "codex")
	p, _ := f.run.team.Agent(a.ID)
	e := core.Envelope{ID: "original", Recipient: a.ID, To: a.Name, From: core.Sender{Kind: core.SenderSelfDeclared, Name: "hitch"}, Purpose: "assignment", Message: core.Message{Text: "Standing contract. Assignment: résumé."}, CreatedAt: f.cmd.now()}
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
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	wire, err := envelopeText(e)
	if err != nil {
		t.Fatal(err)
	}
	b := &clearingCodexInput{inputFixture: f.input}
	b.pasted = "untrusted hidden draft"
	b.screen = screenWithText(fmt.Sprintf("› [Pasted Content %d chars]", utf8.RuneCountInString(wire)))
	b.submit = func(prompt string) error {
		return p.WriteWitness(store.Witness{ID: "recovered", At: f.cmd.now(), Prompt: prompt, SessionID: "s"})
	}
	f.cmd.inputBackend = b
	return f, p, e, b, wire
}

func TestRecoverStartupReplacesCollapsedDraftAfterClear(t *testing.T) {
	f, p, e, b, wire := newCollapsedRecoveryFixture(t)
	if err := f.cmd.hitch([]string{"worker", "--recover"}); err != nil {
		t.Fatal(err)
	}
	got, err := p.ReadEnvelope("cur", e.ID)
	if err != nil || got.Outcome != "delivered" || b.clears != 1 || b.repastes != 1 || b.submits != 1 || b.pasted != wire || !strings.Contains(f.out.String(), "delivered") {
		t.Fatalf("collapsed recovery: receipt=%+v clears=%d repastes=%d submits=%d pasted=%q output=%s err=%v", got, b.clears, b.repastes, b.submits, b.pasted, f.out, err)
	}
}

func TestRecoverStartupKeepsUnknownCollapsedDraftUnsubmitted(t *testing.T) {
	for _, test := range []struct {
		name  string
		setup func(*clearingCodexInput, string)
		clear int
	}{
		{"different count", func(b *clearingCodexInput, wire string) {
			b.screen = screenWithText(fmt.Sprintf("› [Pasted Content %d chars]", utf8.RuneCountInString(wire)+1))
		}, 0},
		{"trailing draft", func(b *clearingCodexInput, wire string) {
			b.screen = screenWithText(fmt.Sprintf("› [Pasted Content %d chars]", utf8.RuneCountInString(wire)), "operator draft")
		}, 0},
		{"clear did not empty", func(b *clearingCodexInput, _ string) { b.clearFails = true }, 1},
		{"clear left another row", func(b *clearingCodexInput, _ string) { b.tailAfterClear = true }, 1},
		{"native prompt after clear", func(b *clearingCodexInput, _ string) { b.promptAfterClear = true }, 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			f, p, e, b, wire := newCollapsedRecoveryFixture(t)
			test.setup(b, wire)
			if err := f.cmd.hitch([]string{"worker", "--recover"}); err == nil {
				t.Fatal("unknown draft recovered")
			}
			got, err := p.ReadEnvelope("failed", e.ID)
			if err != nil || got.Outcome != "unverified" || b.clears != test.clear || b.repastes != 0 || b.submits != 0 {
				t.Fatalf("unsafe recovery: receipt=%+v clears=%d repastes=%d submits=%d err=%v", got, b.clears, b.repastes, b.submits, err)
			}
		})
	}
}

func TestRecoverStartupStopsWhenWitnessChanges(t *testing.T) {
	for _, test := range []struct{ capture, clears, repastes int }{{2, 0, 0}, {3, 1, 0}, {4, 1, 1}} {
		t.Run(fmt.Sprintf("capture %d", test.capture), func(t *testing.T) {
			f, p, e, b, _ := newCollapsedRecoveryFixture(t)
			b.onCapture = func(count int) {
				if count == test.capture {
					if err := p.WriteWitness(store.Witness{ID: "operator-submitted", At: f.cmd.now(), Prompt: "other prompt", SessionID: "s"}); err != nil {
						t.Fatal(err)
					}
				}
			}
			if err := f.cmd.hitch([]string{"worker", "--recover"}); err == nil {
				t.Fatal("changed witness passed")
			}
			got, err := p.ReadEnvelope("failed", e.ID)
			if err != nil || got.Outcome != "unverified" || b.clears != test.clears || b.repastes != test.repastes || b.submits != 0 {
				t.Fatalf("witness race: receipt=%+v clears=%d repastes=%d submits=%d err=%v", got, b.clears, b.repastes, b.submits, err)
			}
		})
	}
}

func TestRecoverStartupRetainsAssignmentWhenRepasteFails(t *testing.T) {
	f, p, e, b, _ := newCollapsedRecoveryFixture(t)
	b.pasteFails = true
	if err := f.cmd.hitch([]string{"worker", "--recover"}); err == nil {
		t.Fatal("failed repaste passed")
	}
	got, err := p.ReadEnvelope("failed", e.ID)
	if err != nil || got.Outcome != "unverified" || b.clears != 1 || b.repastes != 1 || b.submits != 0 {
		t.Fatalf("failed repaste lost assignment: receipt=%+v clears=%d repastes=%d submits=%d err=%v", got, b.clears, b.repastes, b.submits, err)
	}
	if err := f.cmd.hitch([]string{"worker", "--recover"}); err == nil || !strings.Contains(err.Error(), "re-hitch") {
		t.Fatalf("empty composer recovery guidance = %v", err)
	}
}
