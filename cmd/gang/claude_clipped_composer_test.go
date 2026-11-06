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
// submits it after the collar's pause once the start of the message shows
// under the composer's rule, and the submit hook confirms it.
func TestClaudeMessageInClippedComposerIsSubmitted(t *testing.T) {
	rule := strings.Repeat("─", 40)
	empty := screenWithText(rule, "❯ ", rule, "  ? for shortcuts")
	namedEmpty := screenWithText(namedRule("probe main"), "❯ ", rule, "  40k/200k (20%)", "  ⏸ manual mode on · ← 1 agent")
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
		empty substrate.Screen
		paint func(text string) substrate.Screen
	}{
		{name: "closing rule below the pane", empty: empty, paint: func(text string) substrate.Screen {
			return screenWithText(append([]string{"earlier reply", rule}, wrap(text)...)...)
		}},
		{name: "named composer's closing rule below the pane", empty: namedEmpty, paint: func(text string) substrate.Screen {
			return screenWithText(append([]string{"earlier reply", namedRule("probe main")}, wrap(text)...)...)
		}},
		{name: "tall frame without a cursor", empty: empty, paint: func(text string) substrate.Screen {
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
				f.input.screen = test.empty
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

// A screen that never shows the paste, such as a full-screen view the paste
// left behind, gets no submit key: the message stays unverified, as it did
// when delivery required a composer read-back.
func TestClaudePasteNeverShownWithholdsSubmit(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := newStateFixture(t)
		a := f.add(t, "a", "worker", "claude")
		f.env["GANGLINE_HITCH_ID"] = string(a.ID)
		f.input.command = "claude"
		rule := strings.Repeat("─", 40)
		f.input.screen = screenWithText(rule, "❯ ", rule, "  ? for shortcuts")
		f.input.onKeys = func(keys substrate.Keys) error {
			if keys.Text != "" {
				f.input.screen = screenWithText("Transcript view", "  earlier reply", "  ctrl+o to return")
			}
			return nil
		}
		f.cmd.settleInput = nil
		f.run.cmd = f.cmd
		f.cmd.stdin = strings.NewReader("hello")
		if err := f.cmd.send([]string{"worker", "--from", "operator"}); err == nil {
			t.Fatal("send to an unseen paste succeeded")
		}
		if f.input.submits != 0 || !strings.Contains(f.out.String(), "unverified") {
			t.Fatalf("submits=%d output=%s errors=%s", f.input.submits, f.out, f.errOut)
		}
	})
}

// A composer that holds other text in place of the paste, such as a draft
// typed after the paste was cleared, gets neither the submit key nor the
// keys that withdraw a paste.
func TestClaudeReplacedPasteIsNeitherSubmittedNorCleared(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := newStateFixture(t)
		a := f.add(t, "a", "worker", "claude")
		f.env["GANGLINE_HITCH_ID"] = string(a.ID)
		f.input.command = "claude"
		rule := strings.Repeat("─", 40)
		f.input.screen = screenWithText(rule, "❯ ", rule, "  ? for shortcuts")
		var after []string
		pasted := false
		f.input.onKeys = func(keys substrate.Keys) error {
			if keys.Text != "" {
				pasted = true
				f.input.screen = screenWithText(rule, "❯ a draft of my own", rule, "  ? for shortcuts")
			} else if pasted {
				after = append(after, keys.Names...)
			}
			return nil
		}
		f.cmd.settleInput = nil
		f.run.cmd = f.cmd
		f.cmd.stdin = strings.NewReader("hello")
		if err := f.cmd.send([]string{"worker", "--from", "operator"}); err == nil {
			t.Fatal("send into a replaced composer succeeded")
		}
		if f.input.submits != 0 || len(after) != 0 || !strings.Contains(f.out.String(), "unverified") {
			t.Fatalf("submits=%d keys after paste=%q output=%s errors=%s", f.input.submits, after, f.out, f.errOut)
		}
	})
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
		f.input.onKeys = func(keys substrate.Keys) error {
			if keys.Text != "" {
				first, _, _ := strings.Cut(strings.TrimPrefix(keys.Text, "\x1b[200~"), " ")
				f.input.screen = screenWithText(rule, "❯ "+first, rule, "  ? for shortcuts")
			}
			return nil
		}
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
		if !strings.HasPrefix(receipt.Reason, "submit key sent; no submit hook confirmed this message in time") {
			t.Fatalf("reason = %q", receipt.Reason)
		}
	})
}
