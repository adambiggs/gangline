package main

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
)

// currentStamp matches stamp text this version writes. A stored stamp
// without it, or no stamp at all, may belong to input an earlier version
// submitted before an upgrade.
var currentStamp = regexp.MustCompile(`^ctx=\S+ at=\d\d-\d\dT\d\d:\d\d:\d\dZ( |;|$)`)

// receiptMatches reports whether witnessed starts with a wire e may have
// been submitted as, and the bytes outside that envelope. It tries the
// current rendering, then, for a receipt an earlier version may have typed,
// that version's rendering, which named Gangline's senders and put the stamp
// at the start of the body.
func receiptMatches(primitive harness.Invocation, e core.Envelope, witnessed string) (bool, string, error) {
	var outside string
	matched, err := anyReceiptWire(e, func(wire string) (bool, error) {
		matched, rest, err := harness.SubmittedEnvelopeMatches(primitive, wire, witnessed)
		outside = rest
		return matched, err
	})
	if matched && e.ContextStamp == nil {
		// A wire rebuilt without its stored stamp is followed by the
		// stamp line that delivery added; that line is not typed input.
		outside = leadingStampLine.ReplaceAllString(outside, "")
	}
	return matched, outside, err
}

// anyReceiptWire calls match with each wire e may have been submitted as
// until one matches.
func anyReceiptWire(e core.Envelope, match func(string) (bool, error)) (bool, error) {
	wire, err := envelopeText(e)
	if err != nil {
		return false, err
	}
	if matched, err := match(wire); err != nil || matched {
		return matched, err
	}
	if e.ContextStamp != nil && currentStamp.MatchString(e.ContextStamp.Text) {
		return false, nil
	}
	legacy, err := legacyEnvelopeText(e)
	if err != nil || legacy == wire {
		return false, nil
	}
	return match(legacy)
}

var leadingStampLine = regexp.MustCompile(`^\n?\[gang [^\n\]]* /\]`)

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
		return legacyRenderEnvelope(sender, e.Token, e.Purpose, legacyStampedBody(stamp, message))
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
		part, err := legacyRenderEnvelope(section.name, e.Token+"-"+section.name, "startup", legacyStampedBody(occupancy, section.body))
		if err != nil {
			return "", err
		}
		parts = append(parts, part)
	}
	if e.Purpose == "startup" {
		sender = "startup"
	}
	part, err := legacyRenderEnvelope(sender, e.Token, e.Purpose, legacyStampedBody(stamp, message))
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

// legacyRenderEnvelope is the earlier renderer: it defanged only "gang:"
// tags and always framed the body with one space on each side.
func legacyRenderEnvelope(sender, nonce, marker, body string) (string, error) {
	if !envelopeTokenPattern.MatchString(sender) || !envelopeTokenPattern.MatchString(nonce) {
		return "", fmt.Errorf("envelope sender %q or nonce %q is invalid", sender, nonce)
	}
	if err := validateMessageText(body); err != nil {
		return "", err
	}
	if marker != "" && !envelopeMarkerPattern.MatchString(marker) {
		return "", fmt.Errorf("envelope marker %q is invalid", marker)
	}
	head := envelopeHead(sender, nonce)
	body = defangTagsWith(body, func(text string) bool { return envelopeKeywordBefore(text, ":") })
	return fmt.Sprintf("%s %s [/%s]", envelopeOpening(head, marker), body, head), nil
}
