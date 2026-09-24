package main

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/substrate"
)

type collapsedCodexInput struct{ *inputFixture }

func (b collapsedCodexInput) SendKeys(ctx context.Context, pane substrate.PaneID, keys substrate.Keys) error {
	if err := b.inputFixture.SendKeys(ctx, pane, keys); err != nil {
		return err
	}
	if keys.Text != "" {
		b.screen = screenWithText(fmt.Sprintf("› [Pasted Content %d chars]", len(b.pasted)))
	}
	return nil
}

func TestStartupSubmitsCollapsedCodexPaste(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "codex")
	p, _ := f.run.team.Agent(a.ID)
	f.env["GANGLINE_HITCH_ID"] = string(a.ID)
	f.cmd.inputBackend = collapsedCodexInput{f.input}
	f.cmd.settleInput = func(ctx context.Context, b harnessInput, pane substrate.PaneID, c harness.Collar, _ time.Duration) error {
		screen, err := b.Capture(ctx, pane)
		if err != nil {
			return err
		}
		composer, err := harness.ReadComposer(c.Primitives.Composer, screen)
		if err != nil {
			return err
		}
		if !strings.HasPrefix(composer.Text, "[Pasted Content ") {
			return fmt.Errorf("missing collapsed paste: %q", composer.Text)
		}
		return harness.AwaitComposerSettle(ctx, b.Capture, pane, c, 0)
	}
	f.run.cmd = f.cmd
	e := core.Envelope{ID: "startup-collapsed", Recipient: a.ID, To: a.Name, From: core.Sender{Kind: core.SenderGangline, Name: "hitch"}, Purpose: "assignment", Message: core.Message{Text: strings.Repeat("contract and assignment\n", 500)}, CreatedAt: f.cmd.now()}
	if err := p.Publish(e); err != nil {
		t.Fatal(err)
	}
	if err := f.run.tickAgent(a.ID, hookNotice{}, false); err != nil {
		t.Fatal(err)
	}
	got, err := p.ReadEnvelope("cur", e.ID)
	if err != nil || got.Outcome != "delivered" || f.input.submits != 1 {
		t.Fatalf("receipt=%+v submits=%d err=%v", got, f.input.submits, err)
	}
}
