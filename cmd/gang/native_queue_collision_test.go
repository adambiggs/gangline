package main

import (
	"strings"
	"testing"

	"github.com/adambiggs/gangline/core"
)

func TestNativeQueueGuardRejectsReusedTokenForDistinctMessage(t *testing.T) {
	f, a, p := queueSendFixture(t)
	first := core.Envelope{ID: "first", Token: "0123456789abcdef", Recipient: a.ID, To: a.Name, From: core.Sender{Kind: core.SenderSelfDeclared, Name: "operator"}, Message: core.Message{Text: "first message"}, CreatedAt: f.cmd.now()}
	if err := p.Publish(first); err != nil {
		t.Fatal(err)
	}
	f.input.submit = func(wire string) error { f.input.screen = nativeQueueScreen(wire); return nil }
	if outcome, err := f.run.drain(a.ID, first.ID); err != nil || outcome != "accepted" {
		t.Fatalf("first native queue receipt: %q %v", outcome, err)
	}
	// Random tokens have no uniqueness check. A distinct message can receive
	// the same token while the first remains in the native queue.
	second := first
	second.ID, second.Message.Text = "second", "second message"
	if err := p.Publish(second); err != nil {
		t.Fatal(err)
	}
	_, err := f.run.drain(a.ID, second.ID)
	if err == nil || !strings.Contains(err.Error(), "message already appears in native queue before input") {
		t.Fatalf("reused native token was submitted: %v", err)
	}
	if f.input.submits != 1 {
		t.Fatalf("native submits=%d; second message entered", f.input.submits)
	}
	queued, err := p.ReadEnvelope("new", second.ID)
	if err != nil || queued.Token != second.Token || queued.Outcome != "" {
		t.Fatalf("refused message not retained: %+v %v", queued, err)
	}
	got, err := p.Read()
	if err != nil || got.Input != nil {
		t.Fatalf("guard persisted a submission intent: %+v %v", got.Input, err)
	}
}
