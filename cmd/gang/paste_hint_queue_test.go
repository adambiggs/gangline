//go:build acceptance

package main

import (
	"strings"
	"testing"
)

func TestSendExplainsQueueUntilPasteHintFooterReturns(t *testing.T) {
	fixture := newStateFixture(t)
	agent := fixture.add(t, "a", "worker", "claude")
	fixture.env["GANGLINE_HITCH_ID"] = string(agent.ID)
	fixture.input.command = "claude"
	fixture.input.screen = screenWithText(namedRule("Main session"), "❯ ", strings.Repeat("─", 40), "  40k/200k (20%)", "  paste again to expand")
	fixture.run.cmd = fixture.cmd
	fixture.cmd.stdin = strings.NewReader("second message")
	err := fixture.cmd.send([]string{"worker", "--from", "operator"})
	if err != nil || !strings.Contains(fixture.out.String(), "queued") || !strings.Contains(fixture.errOut.String(), "for a tick after the session footer returns") {
		t.Fatalf("error=%v output=%q stderr=%q", err, fixture.out.String(), fixture.errOut.String())
	}
	if fixture.input.pasted != "" || fixture.input.submits != 0 {
		t.Fatalf("text=%q submits=%d", fixture.input.pasted, fixture.input.submits)
	}
}
