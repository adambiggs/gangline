package main

import (
	"regexp"
	"strings"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
)

// currentStamp matches stamp text this version writes. A stored stamp
// without it, or no stamp at all, may belong to input an earlier version
// submitted before an upgrade.
var currentStamp = regexp.MustCompile(`^ctx=\S+ at=\d\d-\d\dT\d\d:\d\d:\d\dZ(;|$)`)

// receiptWires is every wire e may have been submitted as: the current
// rendering and, for a receipt an earlier version may have typed, that
// version's rendering, which named Gangline's senders and put the stamp
// at the start of the body.
func receiptWires(e core.Envelope) ([]string, error) {
	wire, err := envelopeText(e)
	if err != nil {
		return nil, err
	}
	wires := []string{wire}
	if e.ContextStamp == nil || !currentStamp.MatchString(e.ContextStamp.Text) {
		if legacy, err := legacyEnvelopeText(e); err == nil && legacy != wire {
			wires = append(wires, legacy)
		}
	}
	return wires, nil
}

// receiptMatches reports whether witnessed starts with a wire e may have
// been submitted as, and the bytes outside that envelope. Gangline's own
// stamp line is not outside input.
func receiptMatches(primitive harness.Invocation, e core.Envelope, witnessed string) (bool, string, error) {
	wires, err := receiptWires(e)
	if err != nil {
		return false, "", err
	}
	for _, wire := range wires {
		matched, outside, err := harness.SubmittedEnvelopeMatches(primitive, wire, witnessed)
		if err != nil || matched {
			return matched, withoutStampLine(outside), err
		}
	}
	return false, "", nil
}

var leadingStampLine = regexp.MustCompile(`^\n?\[gang [^\n]* /\]`)

// withoutStampLine drops a stamp line that follows a wire rebuilt without
// its stored stamp.
func withoutStampLine(outside string) string {
	return leadingStampLine.ReplaceAllString(outside, "")
}

func legacyEnvelopeText(e core.Envelope) (string, error) {
	sender := senderLabel(e.From)
	message := e.Message.Text
	stamp := ""
	if e.ContextStamp != nil {
		stamp = e.ContextStamp.Text
		if isContextBandNotice(e) {
			message = ""
		}
	}
	if e.Startup == nil {
		return renderEnvelope(sender, e.Token, e.Purpose, legacyStampedBody(stamp, message))
	}
	occupancy, _, _ := strings.Cut(stamp, ";")
	var parts []string
	for _, section := range []struct{ name, body string }{
		{"contract", e.Startup.Contract},
		{"doctrine", e.Startup.Doctrine},
		{"role", e.Startup.Role},
	} {
		if section.body == "" {
			continue
		}
		part, err := renderEnvelope(section.name, e.Token+"-"+section.name, "startup", legacyStampedBody(occupancy, section.body))
		if err != nil {
			return "", err
		}
		parts = append(parts, part)
	}
	if e.Purpose == "startup" {
		sender = "startup"
	}
	part, err := renderEnvelope(sender, e.Token, e.Purpose, legacyStampedBody(stamp, message))
	if err != nil {
		return "", err
	}
	return strings.Join(append(parts, part), "\n\n"), nil
}

func legacyStampedBody(stamp, body string) string {
	if stamp == "" || body == "" {
		return stamp + body
	}
	return stamp + " " + body
}
