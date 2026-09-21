package main

import (
	"fmt"
	"regexp"
)

var envelopeTokenPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]*$`)
var envelopeMarkerPattern = regexp.MustCompile(`^[a-z][a-z-]*$`)
var tagShapedTextPattern = regexp.MustCompile(`(?i)[\[［〔【]([[:space:]]*/?[[:space:]]*gang[[:space:]]*:)`)

func renderEnvelope(sender, nonce, marker, body string) (string, error) {
	if !envelopeTokenPattern.MatchString(sender) {
		return "", fmt.Errorf("envelope sender %q is invalid", sender)
	}
	if !envelopeTokenPattern.MatchString(nonce) {
		return "", fmt.Errorf("envelope nonce %q is invalid", nonce)
	}
	if marker != "" && !envelopeMarkerPattern.MatchString(marker) {
		return "", fmt.Errorf("envelope marker %q is invalid", marker)
	}
	suffix := ""
	if marker != "" {
		suffix = " " + marker
	}
	body = tagShapedTextPattern.ReplaceAllString(body, "$1")
	return fmt.Sprintf("[gang:%s#%s%s] %s [/gang:%s#%s]", sender, nonce, suffix, body, sender, nonce), nil
}
