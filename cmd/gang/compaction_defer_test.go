package main

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/store"
	"github.com/adambiggs/gangline/substrate"
)

type replacedCompactDraft struct {
	*inputFixture
	replaced bool
	draft    string
}

func (b *replacedCompactDraft) Capture(ctx context.Context, pane substrate.PaneID) (substrate.Screen, error) {
	screen, err := b.inputFixture.Capture(ctx, pane)
	if !b.replaced {
		c, _ := harness.EmbeddedCollar("codex")
		composer, readErr := harness.ReadComposer(c.Primitives.Composer, screen)
		busy, _ := harness.Busy(c, screen)
		if readErr == nil && composer.Text == "/compact" && busy {
			b.screen = screenWithText(append([]string{"Working (esc to interrupt)"}, strings.Split("› "+b.draft, "\n")...)...)
			b.replaced = true
		}
	}
	return screen, err
}

func TestCompactionDeferLeavesChangedComposer(t *testing.T) {
	for _, draft := range []string{"operator draft", "/com pact", "/compact\noperator continuation"} {
		t.Run(draft, func(t *testing.T) {
			f, _, p := compactionFixture(t)
			b := &replacedCompactDraft{inputFixture: f.input, draft: draft}
			f.cmd.inputBackend, f.input.registeredSender = b, b
			f.cmd.settleInput = func(context.Context, harnessInput, substrate.PaneID, harness.Collar, time.Duration) error {
				f.input.screen = screenWithText("Working (esc to interrupt)", "› /compact")
				return nil
			}
			_ = f.cmd.compact([]string{"worker"})
			a, err := p.Read()
			if err != nil || a.Compaction.Status != "failed" || !strings.Contains(a.Compaction.Reason, "no longer identified") || slices.Contains(f.input.keys, "C-u") || f.input.submits != 0 {
				t.Fatalf("changed draft: compaction=%+v keys=%v submits=%d err=%v", a.Compaction, f.input.keys, f.input.submits, err)
			}
			c, _ := harness.EmbeddedCollar("codex")
			composer, _ := harness.ReadComposer(c.Primitives.Composer, b.screen)
			if composer.Text != strings.Split(draft, "\n")[0] {
				t.Fatalf("foreign draft changed: %q", composer.Text)
			}
		})
	}
}

func TestCompactionDeferRequiresConfirmedWithdrawal(t *testing.T) {
	f, _, p := compactionFixture(t)
	f.cmd.settleInput = func(context.Context, harnessInput, substrate.PaneID, harness.Collar, time.Duration) error {
		f.input.screen = screenWithText("Working (esc to interrupt)", "› /compact")
		return nil
	}
	_ = f.cmd.compact([]string{"worker", "--resume", "saved original note"})
	a, err := p.Read()
	if err != nil || a.Compaction.Status != "failed" || !strings.Contains(a.Compaction.Reason, "withdrawal unconfirmed") || a.Compaction.Resume.Text != "saved original note" || !slices.Equal(f.input.keys, []string{"C-u"}) || f.input.submits != 0 {
		t.Fatalf("unconfirmed withdrawal: compaction=%+v keys=%v submits=%d err=%v", a.Compaction, f.input.keys, f.input.submits, err)
	}
}

func TestFailedDeferredCompactDraftCanBeClearedExplicitly(t *testing.T) {
	f, a, p := compactionFixture(t)
	a.Compaction = &core.Compaction{
		ID: "failed-compact", Status: "failed", Resume: core.Message{Text: "resume from saved state"},
		Reason: "compaction not submitted: native task became active before compaction submit; staged command no longer identified in composer; input left alone",
	}
	l, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Save(a); err != nil {
		t.Fatal(err)
	}
	queued := core.Envelope{
		ID: "queued-after-compact", Token: "0123456789abcdef", Recipient: a.ID, To: a.Name,
		From:    core.Sender{Kind: core.SenderGangline, Name: "test"},
		Message: core.Message{Text: "continue queued work"}, CreatedAt: f.cmd.now(),
	}
	if err := f.run.publishOnce(l, &a, queued); err != nil {
		t.Fatal(err)
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	compactScreen := func(text string) substrate.Screen {
		screen := screenWithText("› "+text, "", "  GPT-6-Astra high · Context 4% used · gangline · never")
		screen.Cursor = substrate.Cursor{Row: 0, Column: 2 + len(text), Visible: true}
		return screen
	}
	f.input.screen = compactScreen("/compact")
	f.input.submit = func(prompt string) error {
		return p.WriteWitness(store.Witness{ID: "queued-witness", At: f.cmd.now(), Prompt: prompt, SessionID: "s"})
	}
	f.input.onKeys = func(keys substrate.Keys) error {
		if slices.Contains(keys.Names, "C-u") {
			f.input.screen = compactScreen("")
		}
		return nil
	}
	f.cmd.settleInput = func(context.Context, harnessInput, substrate.PaneID, harness.Collar, time.Duration) error { return nil }
	f.run.cmd = f.cmd
	if err := f.cmd.compact([]string{"worker", "--interrupt"}); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(f.input.keys, []string{"C-u"}) || f.input.submits != 1 || !strings.Contains(f.out.String(), "failed compact draft cleared") {
		t.Fatalf("recovery keys=%v submits=%d output=%q", f.input.keys, f.input.submits, f.out.String())
	}
	receipt, err := p.ReadEnvelope("cur", queued.ID)
	if err != nil || receipt.Outcome != "delivered" {
		t.Fatalf("queued work receipt: %+v, %v", receipt, err)
	}
	got, err := p.Read()
	if err != nil || got.Compaction.Status != "failed" {
		t.Fatalf("failed compaction changed: %+v, %v", got.Compaction, err)
	}
}

func TestFailedDeferredCompactDraftLeavesOtherInputAlone(t *testing.T) {
	for _, tc := range []struct {
		name, reason string
		screen       substrate.Screen
	}{
		{name: "edited", screen: screenWithText("› /compact extra")},
		{name: "busy", screen: screenWithText("Working (esc to interrupt)", "› /compact")},
		{name: "blocked", screen: screenWithText("Do you want to allow this command?", "1. Yes, proceed", "› /compact")},
		{name: "unrelated failure", reason: "compaction not submitted: composer unread", screen: screenWithText("› /compact")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, a, p := compactionFixture(t)
			reason := tc.reason
			if reason == "" {
				reason = "compaction not submitted: native task became active before compaction submit; staged command no longer identified in composer; input left alone"
			}
			a.Compaction = &core.Compaction{ID: "failed-compact", Status: "failed", Reason: reason}
			l, err := p.TryLock()
			if err != nil {
				t.Fatal(err)
			}
			if err := l.Save(a); err != nil {
				t.Fatal(err)
			}
			if err := l.Close(); err != nil {
				t.Fatal(err)
			}
			f.input.screen = tc.screen
			f.run.cmd = f.cmd
			if err := f.cmd.compact([]string{"worker", "--interrupt"}); err == nil {
				t.Fatal("foreign or blocked input was accepted for recovery")
			}
			if len(f.input.keys) != 0 || f.input.submits != 0 {
				t.Fatalf("input changed: keys=%v submits=%d", f.input.keys, f.input.submits)
			}
		})
	}
}
