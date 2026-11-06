//go:build acceptance

package main

import (
	"context"
	"strings"
	"testing"
	"testing/synctest"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/substrate"
)

// A message pasted into the empty composer of a short pane can leave a screen
// on which the composer cannot be read back: its closing rule below the pane,
// or a tall frame whose cursor the screen does not show. Delivery still
// submits it after the collar's pause, and the submit hook confirms it.
func TestClaudeMessageInClippedComposerIsSubmitted(t *testing.T) {
	rule := strings.Repeat("─", 40)
	wrap := func(text string) []string {
		lines := []string{"❯ " + text[:36]}
		for remaining := text[36:]; remaining != ""; {
			n := min(36, len(remaining))
			lines = append(lines, "  "+remaining[:n])
			remaining = remaining[n:]
		}
		return lines
	}
	for _, test := range []struct {
		name  string
		paint func(text string) substrate.Screen
	}{
		{name: "closing rule below the pane", paint: func(text string) substrate.Screen {
			return screenWithText(append([]string{"earlier reply", rule}, wrap(text)...)...)
		}},
		{name: "tall frame without a cursor", paint: func(text string) substrate.Screen {
			lines := append([]string{rule}, wrap(text)...)
			return screenWithText(append(lines, rule, "  ? for shortcuts")...)
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				f := newStateFixture(t)
				a := f.add(t, "a", "worker", "claude")
				f.env["GANGLINE_HITCH_ID"] = string(a.ID)
				f.input.command = "claude"
				f.input.screen = screenWithText(rule, "❯ ", rule, "  ? for shortcuts")
				f.input.onKeys = func(keys substrate.Keys) error {
					if keys.Text != "" {
						f.input.screen = test.paint(strings.TrimSuffix(strings.TrimPrefix(keys.Text, "\x1b[200~"), "\x1b[201~"))
					}
					return nil
				}
				f.cmd.settleInput = nil // Exercise the production pause under fake time.
				f.run.cmd = f.cmd
				f.cmd.stdin = strings.NewReader("a message long enough to wrap the composer of a short pane past its bottom edge")
				if err := f.cmd.send([]string{"worker", "--from", "operator"}); err != nil {
					t.Fatalf("%v; output=%s errors=%s", err, f.out, f.errOut)
				}
				if f.input.submits != 1 || !strings.Contains(f.out.String(), "delivered") {
					t.Fatalf("submits=%d output=%s errors=%s", f.input.submits, f.out, f.errOut)
				}
			})
		})
	}
}

// When the submit hook never reports a submitted message, the receipt says
// the submit key was sent, so the sender knows the input is not left waiting
// in the composer for a key.
func TestClaudeSubmitWithoutHookReportsStall(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := newStateFixture(t)
		a := f.add(t, "a", "worker", "claude")
		f.env["GANGLINE_HITCH_ID"] = string(a.ID)
		f.input.command = "claude"
		rule := strings.Repeat("─", 40)
		f.input.screen = screenWithText(rule, "❯ ", rule, "  ? for shortcuts")
		f.input.submit = func(string) error { return nil } // no submit hook fires
		// The witness wait reaches the operation deadline without a change.
		f.cmd.newWatch = func(string) (changeWait, error) {
			return waitFixture{func(context.Context) error { return context.DeadlineExceeded }}, nil
		}
		f.cmd.settleInput = nil
		f.run.cmd = f.cmd
		f.cmd.stdin = strings.NewReader("hello")
		if err := f.cmd.send([]string{"worker", "--from", "operator"}); err == nil {
			t.Fatal("send without a submit hook succeeded")
		}
		fields := strings.Fields(f.out.String())
		if f.input.submits != 1 || len(fields) != 2 || fields[1] != "unverified" {
			t.Fatalf("submits=%d output=%s errors=%s", f.input.submits, f.out, f.errOut)
		}
		p, err := f.run.team.Agent(a.ID)
		if err != nil {
			t.Fatal(err)
		}
		receipt, err := p.ReadEnvelope("failed", core.EnvelopeID(fields[0]))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(receipt.Reason, "submit key sent but the harness reported no submitted prompt in time") {
			t.Fatalf("reason = %q", receipt.Reason)
		}
	})
}
