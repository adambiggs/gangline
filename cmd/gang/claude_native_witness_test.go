package main

import (
	"strings"
	"testing"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
)

func TestClaudePeerWitnessRequiresTheCompleteNativeBody(t *testing.T) {
	e := core.Envelope{Token: "0123456789abcdef", From: core.Sender{Kind: core.SenderSelfDeclared, Name: "probe"}, Message: core.Message{Text: "first line\nsecond line λ"}}
	wire, err := envelopeText(e)
	if err != nil {
		t.Fatal(err)
	}
	body := claudePeerBody(wire)
	for _, tc := range []struct {
		name, prompt, primitive string
		want                    bool
	}{
		{"native body", body, "claude-pasted-content", true},
		{"raw fallback", wire, "claude-pasted-content", true},
		{"clipped opener and envelope", strings.TrimPrefix(body, "<cross-session-message>\n[gang:"), "claude-pasted-content", false},
		{"clipped body", claudePeerBody(wire[:len(wire)/2]), "claude-pasted-content", false},
		{"extra text", body + "keyboard text", "claude-pasted-content", false},
		{"changed sender", strings.Replace(body, "self-declared:probe", "self-declared:other", 1), "claude-pasted-content", false},
		{"changed token", strings.ReplaceAll(body, e.Token, "fedcba9876543210"), "claude-pasted-content", false},
		{"asserted metadata", strings.Replace(body, "<cross-session-message>", "<cross-session-message from=\"other\">", 1), "claude-pasted-content", false},
		{"other harness", body, "exact-prompt", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, outside, err := receiptMatches(harness.Invocation{Name: tc.primitive}, e, tc.prompt)
			if err != nil || got != tc.want || outside != "" {
				t.Fatalf("match=%t outside=%q error=%v; want %t and no outside bytes", got, outside, err, tc.want)
			}
		})
	}
}
