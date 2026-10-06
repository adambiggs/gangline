package main

import (
	"strings"
	"testing"

	"github.com/adambiggs/gangline/core"
)

// The reported resumed lead witness lost the two newlines after Assignment:.
// Replay that loss through delivery, including a later completion report.
func TestClaudeResumedStartupWithNativeNewlineLoss(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "a", "lead", "claude")
	p, err := f.run.team.Agent(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	a.Status, a.Activity = core.Booting, core.Unknown
	a.Native.SessionID = "s"
	l, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Save(a); err != nil {
		t.Fatal(err)
	}
	l.Close()
	_, message := startupMessages("lead", startupProse{Contract: []byte("standing contract")}, "This is a resumed session. Re-read your brief and durable state, then continue your work or report that you are waiting for an assignment.", true)
	e := core.Envelope{ID: "startup-resume", Token: "0123456789abcdef", Recipient: a.ID, To: a.Name, From: core.Sender{Kind: core.SenderGangline, Name: "hitch"}, Purpose: "assignment", Message: core.Message{Text: message}, CreatedAt: f.cmd.now()}
	if err := p.Publish(e); err != nil {
		t.Fatal(err)
	}
	f.env["GANGLINE_HITCH_ID"] = string(a.ID)
	f.input.command = "claude"
	f.input.screen = screenWithText("────────────────", "❯ ", "────────────────")
	submit := f.input.submit
	f.input.submit = func(prompt string) error {
		witness := strings.ReplaceAll(prompt, "\n", "")
		t.Logf("sent %d bytes, witnessed %d bytes", len(prompt), len(witness))
		return submit(witness)
	}
	if err := f.run.tickAgent(a.ID, hookNotice{}, false); err != nil {
		t.Fatal(err)
	}
	got, err := p.ReadEnvelope("cur", e.ID)
	if err != nil || got.Outcome != "delivered" {
		failed, _ := p.ReadEnvelope("failed", e.ID)
		t.Fatalf("startup receipt=%+v, failed=%+v, err=%v", got, failed, err)
	}
	f.cmd.stdin = strings.NewReader("The change is complete.")
	if err := f.cmd.send([]string{"lead", "--from", "owner"}); err != nil {
		t.Fatal(err)
	}
	if f.input.submits != 2 || !strings.Contains(f.out.String(), "delivered") {
		t.Fatalf("completion report held: submits=%d output=%s errors=%s", f.input.submits, f.out, f.errOut)
	}
}
