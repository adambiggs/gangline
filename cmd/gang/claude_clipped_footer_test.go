package main

import (
	"strings"
	"testing"
	"testing/synctest"

	"github.com/adambiggs/gangline/substrate"
)

// namedRule draws the rule above a named Claude Code composer in a 40-column
// pane.
func namedRule(label string) string {
	return strings.Repeat("─", 37-len([]rune(label))) + " " + label + " ─"
}

// A message that wraps a named main session's composer pushes its mode line
// off a short pane. The rule above the composer still names the session that
// owned it before the paste.
func TestClaudeWrappedMessageWithClippedFooterReachesSubmit(t *testing.T) {
	for _, test := range []struct {
		name, label string
		submits     int
		outcome     string
	}{
		{name: "same main", label: "probe main", submits: 1, outcome: "delivered"},
		{name: "child view", label: "Read the harness sources", submits: 0, outcome: "unverified"},
	} {
		t.Run(test.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				f := newStateFixture(t)
				a := f.add(t, "a", "worker", "claude")
				f.env["GANGLINE_HITCH_ID"] = string(a.ID)
				f.input.command = "claude"
				closing := strings.Repeat("─", 40)
				f.input.screen = screenWithText(namedRule("probe main"), "❯ ", closing, "  40k/200k (20%)", "  ⏸ manual mode on · ← 1 agent")
				f.input.onKeys = func(keys substrate.Keys) error {
					if keys.Text == "" {
						return nil
					}
					text := strings.TrimSuffix(strings.TrimPrefix(keys.Text, "\x1b[200~"), "\x1b[201~")
					lines := []string{namedRule(test.label), "❯ " + text[:36]}
					for remaining := text[36:]; remaining != ""; {
						n := min(36, len(remaining))
						lines = append(lines, "  "+remaining[:n])
						remaining = remaining[n:]
					}
					f.input.screen = screenWithText(append(lines, closing, "  40k/200k (20%)")...)
					return nil
				}
				f.cmd.settleInput = nil // Exercise the production settle loop under fake time.
				f.run.cmd = f.cmd
				f.cmd.stdin = strings.NewReader("a single paragraph long enough to wrap the composer")
				err := f.cmd.send([]string{"worker", "--from", "operator"})
				if test.outcome == "delivered" && err != nil {
					t.Fatal(err)
				}
				if f.input.submits != test.submits || !strings.Contains(f.out.String(), test.outcome) {
					t.Fatalf("submits=%d output=%s errors=%s err=%v", f.input.submits, f.out, f.errOut, err)
				}
			})
		})
	}
}
