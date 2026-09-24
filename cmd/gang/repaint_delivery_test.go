package main

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/store"
	"github.com/adambiggs/gangline/substrate"
)

type repaintInput struct {
	*inputFixture
	missing bool
}

func (b *repaintInput) SendKeys(ctx context.Context, pane substrate.PaneID, keys substrate.Keys) error {
	if err := b.inputFixture.SendKeys(ctx, pane, keys); err != nil {
		return err
	}
	if keys.Text != "" {
		b.screen = screenWithText("› " + b.pasted)
		b.missing = true
	}
	return nil
}

func (b *repaintInput) Capture(ctx context.Context, pane substrate.PaneID) (substrate.Screen, error) {
	if b.missing {
		b.missing = false
		return screenWithText("repainting"), nil
	}
	return b.inputFixture.Capture(ctx, pane)
}

func TestRepaintDeliveryAllowsQueuedCompactionAndMessages(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := newStateFixture(t)
		a := f.add(t, "a", "worker", "codex")
		p, _ := f.run.team.Agent(a.ID)
		f.env["GANGLINE_HITCH_ID"] = string(a.ID)
		f.input.screen = screenWithText("Working (esc to interrupt)", "› ")
		f.cmd.inputBackend = &repaintInput{inputFixture: f.input}
		f.cmd.settleInput = nil // Exercise the production settle loop under fake time.
		f.cmd.newWatch = func(string) (changeWait, error) {
			return waitFixture{func(context.Context) error { return fmt.Errorf("missing synchronous witness") }}, nil
		}
		f.run.cmd = f.cmd
		var prompts []string
		f.input.submit = func(prompt string) error {
			prompts = append(prompts, prompt)
			f.input.screen = screenWithText("› ")
			if prompt == "/compact" {
				return nil
			}
			return p.WriteWitness(store.Witness{ID: fmt.Sprint(len(prompts)), At: f.cmd.now(), Prompt: prompt, SessionID: "s"})
		}
		f.cmd.stdin = strings.NewReader("steer the running turn")
		if err := f.cmd.send([]string{"worker", "--from", "operator"}); err != nil {
			t.Fatal(err)
		}
		if f.input.submits != 1 || !strings.Contains(f.out.String(), "delivered") {
			t.Fatalf("mid-turn receipt: %s, submits=%d", f.out, f.input.submits)
		}

		f.input.screen = screenWithText("Working (esc to interrupt)", "› ")
		if err := f.cmd.compact([]string{"worker", "--resume", "resume the work"}); err != nil {
			t.Fatal(err)
		}
		for i := range 3 {
			e := core.Envelope{ID: core.EnvelopeID(fmt.Sprintf("queued-%d", i)), Recipient: a.ID, To: a.Name, From: core.Sender{Kind: core.SenderSelfDeclared, Name: "operator"}, Message: core.Message{Text: fmt.Sprintf("followup %d", i)}, CreatedAt: f.cmd.now().Add(time.Duration(i) * time.Second)}
			if err := p.Publish(e); err != nil {
				t.Fatal(err)
			}
		}
		f.input.screen = screenWithText("› ")
		if err := f.run.tickAgent(a.ID, hookNotice{Kind: "turn-finished", SessionID: "s", At: f.cmd.now()}, false); err != nil {
			t.Fatal(err)
		}
		if len(prompts) != 2 || prompts[1] != "/compact" {
			t.Fatalf("idle boundary did not submit only compaction: %q", prompts)
		}
		if err := f.run.tickAgent(a.ID, hookNotice{Kind: "compaction-finished", SessionID: "s", At: f.cmd.now().Add(time.Second)}, false); err != nil {
			t.Fatal(err)
		}
		if len(prompts) != 6 || !strings.Contains(prompts[2], "resume the work") {
			t.Fatalf("resume and queued messages not drained: %q", prompts)
		}
		for i := range 3 {
			if !strings.Contains(prompts[i+3], fmt.Sprintf("followup %d", i)) {
				t.Fatalf("queued message %d out of order: %q", i, prompts)
			}
		}
		last, err := p.ReadEnvelope("cur", "queued-2")
		if err != nil || last.Outcome != "delivered" {
			t.Fatalf("final receipt: %+v, %v", last, err)
		}
		pending, err := p.ListNew()
		if err != nil || len(pending) != 0 {
			t.Fatalf("remaining spool: %+v, %v", pending, err)
		}
	})
}
