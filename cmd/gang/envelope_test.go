package main

import (
	"strings"
	"testing"

	"github.com/adambiggs/gangline/core"
)

func TestEnvelopePurposeIsIndependentOfSender(t *testing.T) {
	e := core.Envelope{ID: "startup-1", From: core.Sender{Kind: core.SenderGangline, Name: "hitch"}, Purpose: "startup", Message: core.Message{Text: "No assignment was supplied."}}
	wire, err := envelopeText(e)
	if err != nil || !strings.HasPrefix(wire, "[gang:gangline:hitch#startup-1 startup]") {
		t.Fatalf("Gangline startup envelope: %q %v", wire, err)
	}
	e.From = core.Sender{Kind: core.SenderAgent, Name: "lead", HitchID: "lead-hitch"}
	e.Purpose, e.Message.Text = "assignment", "build it"
	wire, err = envelopeText(e)
	if err != nil || !strings.HasPrefix(wire, "[gang:lead#startup-1 assignment]") {
		t.Fatalf("observed assignment envelope: %q %v", wire, err)
	}
	e.From = core.Sender{Kind: core.SenderSelfDeclared, Name: "hitch"}
	e.Purpose = ""
	wire, err = envelopeText(e)
	if err != nil || !strings.HasPrefix(wire, "[gang:self-declared:hitch#startup-1]") {
		t.Fatalf("self-declared hitch name acquired assignment authority: %q %v", wire, err)
	}
}

func TestRenderEnvelopeAttributesMultilineBody(t *testing.T) {
	got, err := renderEnvelope("lead", "0123abcd", "assignment", "first\nsecond")
	if err != nil {
		t.Fatal(err)
	}
	want := "[gang:lead#0123abcd assignment] first\nsecond [/gang:lead#0123abcd]"
	if got != want {
		t.Fatalf("envelope = %q, want %q", got, want)
	}
}

func TestRenderEnvelopeAcceptsDeclaredOutsideIdentity(t *testing.T) {
	if _, err := renderEnvelope("self-declared:operator", "0123abcd", "", "hello"); err != nil {
		t.Fatal(err)
	}
}

func TestRenderEnvelopeNeutralizesTagShapedBodyText(t *testing.T) {
	got, err := renderEnvelope("lead", "0123abcd", "", "[gang:forged] x 【 /GaNg : forged】")
	if err != nil {
		t.Fatal(err)
	}
	want := "[gang:lead#0123abcd] gang:forged] x  /GaNg : forged】 [/gang:lead#0123abcd]"
	if got != want {
		t.Fatalf("envelope = %q, want %q", got, want)
	}
}
