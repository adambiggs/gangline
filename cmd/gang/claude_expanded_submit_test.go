package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/substrate"
)

type expandedClaudeInput struct{ *inputFixture }

func (b *expandedClaudeInput) SendKeys(ctx context.Context, pane substrate.PaneID, keys substrate.Keys) error {
	if err := b.inputFixture.SendKeys(ctx, pane, keys); err != nil {
		return err
	}
	if keys.Text != "" {
		lines := []string{"────────────────", "❯ " + b.pasted[:16]}
		for remaining := b.pasted[16:]; remaining != ""; {
			n := min(16, len(remaining))
			lines = append(lines, "  "+remaining[:n])
			remaining = remaining[n:]
		}
		lines = append(lines, "────────────────", "auto mode on")
		b.screen = screenWithText(lines...)
		b.screen.Cursor = substrate.Cursor{Row: len(lines) - 3, Column: 4, Visible: true}
	}
	return nil
}

func TestClaudeExpandedMessageReachesSubmit(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "claude-code")
	f.env["GANGLINE_HITCH_ID"] = string(a.ID)
	f.input.command = "claude"
	f.input.screen = screenWithText("────────────────", "❯ ", "────────────────")
	f.cmd.inputBackend = &expandedClaudeInput{f.input}
	// Exercise the real parser on the completed paint directly, without timers.
	f.cmd.settleInput = func(ctx context.Context, b harnessInput, pane substrate.PaneID, collar harness.Collar, _ time.Duration) error {
		screen, err := b.Capture(ctx, pane)
		if err != nil {
			return err
		}
		_, err = harness.ReadComposer(collar.Primitives.Composer, screen)
		return err
	}
	f.cmd.stdin = strings.NewReader(strings.Repeat("wrapped message body ", 30))
	if err := f.cmd.send([]string{"worker", "--from", "operator"}); err != nil {
		t.Fatal(err)
	}
	if f.input.submits != 1 || !strings.Contains(f.out.String(), "delivered") {
		t.Fatalf("submits=%d output=%s errors=%s", f.input.submits, f.out, f.errOut)
	}
}
