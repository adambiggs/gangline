package main

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/adambiggs/gangline/harness"
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
