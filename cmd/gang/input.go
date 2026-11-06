package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

const maximumMessageBytes = 1 << 20

// The prompt budget includes its JSON string encoding and Gangline envelope.
// The hook object also carries native metadata and may wrap a pasted prompt.
const maximumHookBytes = 2 * maximumMessageBytes

func readBody(reader io.Reader) (string, error) {
	limited := io.LimitReader(reader, maximumMessageBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return "", fmt.Errorf("read message: %w", err)
	}
	if len(data) > maximumMessageBytes {
		return "", refuseError("message exceeds the %d-byte encoded envelope budget; put details in a state file and send its path", maximumMessageBytes)
	}
	body := strings.TrimSuffix(string(data), "\n")
	if body == "" {
		return "", refuseError("message body is empty")
	}
	if err := validateMessageText(body); err != nil {
		return "", err
	}
	return body, nil
}

func validateMessageText(body string) error {
	if !utf8.ValidString(body) || strings.ContainsFunc(body, func(r rune) bool { return unicode.IsControl(r) && r != '\n' && r != '\t' }) {
		return refuseError("message body must be UTF-8 text without terminal controls other than newline and tab")
	}
	previousBracket, marks := false, false
	for _, r := range body {
		if invisibleMessageRune(r) {
			return refuseError("message body contains U+%04X; remove invisible format characters and line and paragraph separators", r)
		}
		if unicode.IsMark(r) {
			if previousBracket {
				return refuseError("message body contains combining marks adjacent to a bracket")
			}
			marks = true
			continue
		}
		bracket := envelopeBracket(r)
		if bracket && marks {
			return refuseError("message body contains combining marks adjacent to a bracket")
		}
		previousBracket, marks = bracket, false
	}
	return nil
}

func invisibleMessageRune(r rune) bool {
	return unicode.Is(unicode.Cf, r) || unicode.Is(unicode.Zl, r) || unicode.Is(unicode.Zp, r)
}

func randomID(prefix string) (string, error) {
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", fmt.Errorf("create %s id: %w", prefix, err)
	}
	return prefix + "-" + hex.EncodeToString(random[:]), nil
}

func randomEnvelopeToken() (string, error) {
	var random [8]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", fmt.Errorf("create envelope token: %w", err)
	}
	return hex.EncodeToString(random[:]), nil
}

var envelopeTokenPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]*$`)
var envelopeMarkerPattern = regexp.MustCompile(`^[a-z][a-z-]*$`)

// Non-ASCII letters in the keyword positions are treated as lookalikes;
// refusing the shape does not depend on a confusable-letter table.
func envelopeKeyword(text string) bool {
	text = strings.TrimLeftFunc(text, unicode.IsSpace)
	text = strings.TrimPrefix(text, "/")
	text = strings.TrimLeftFunc(text, unicode.IsSpace)
	for _, expected := range "gang" {
		r, size := utf8.DecodeRuneInString(text)
		if r != expected && !(r > 127 && unicode.IsLetter(r)) {
			return false
		}
		text = text[size:]
	}
	text = strings.TrimLeftFunc(text, unicode.IsSpace)
	return strings.HasPrefix(text, ":") || strings.HasPrefix(text, "#")
}

func openingEnvelopeBracket(r rune) bool {
	return unicode.Is(unicode.Ps, r) || strings.ContainsRune("⎡⎢⎣", r)
}

func envelopeBracket(r rune) bool {
	return openingEnvelopeBracket(r) || unicode.Is(unicode.Pe, r) || strings.ContainsRune("⎤⎥⎦", r)
}

func canonicalEnvelopeText(text string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case unicode.IsMark(r), strings.ContainsRune("\u115f\u1160\u2800", r):
			return -1
		case unicode.IsSpace(r):
			return ' '
		case strings.ContainsRune("∕⁄⧸", r):
			return '/'
		case strings.ContainsRune("꞉∶։ː˸", r):
			return ':'
		default:
			return unicode.ToLower(r)
		}
	}, norm.NFKC.String(text))
}

func defangEnvelopeTags(body string) string {
	type prefix struct{ start, end, slashes int }
	var pending []prefix
	var result strings.Builder
	copied := 0
	for i, r := range body {
		if !envelopeBracket(r) {
			continue
		}
		if !openingEnvelopeBracket(r) {
			pending = nil
			continue
		}
		next := i + utf8.RuneLen(r)
		candidate := body[next:]
		if end := strings.IndexFunc(candidate, envelopeBracket); end >= 0 {
			candidate = candidate[:end]
		}
		candidate = canonicalEnvelopeText(candidate)
		trimmed := strings.TrimSpace(candidate)
		slash := 0
		if strings.HasPrefix(trimmed, "/") {
			slash = 1
		}
		if envelopeKeyword(candidate) {
			// Remove enclosing prefixes too: deleting only the inner bracket exposes a new tag.
			first := len(pending)
			for first > 0 && slash+pending[first-1].slashes <= 1 {
				first--
				slash += pending[first].slashes
			}
			for _, p := range pending[first:] {
				result.WriteString(body[copied:p.start])
				copied = p.end
			}
			result.WriteString(body[copied:i])
			copied = next
			pending = nil
		} else if trimmed == "" || trimmed == "/" {
			pending = append(pending, prefix{i, next, slash})
		} else {
			pending = nil
		}
	}
	if copied == 0 {
		return body
	}
	result.WriteString(body[copied:])
	return result.String()
}

// renderEnvelope wraps body in tags naming sender and nonce.
func renderEnvelope(sender, nonce, marker, body string) (string, error) {
	if !envelopeTokenPattern.MatchString(sender) {
		return "", fmt.Errorf("envelope sender %q is invalid", sender)
	}
	return renderEnvelopeHead(sender, nonce, marker, body)
}

// renderGanglineEnvelope wraps a message Gangline itself emits in tags that
// name no sender: [gang#NONCE] ... [/gang#NONCE]. Only Gangline's own senders
// use it, so no agent or outside name can render this form.
func renderGanglineEnvelope(nonce, marker, body string) (string, error) {
	return renderEnvelopeHead("", nonce, marker, body)
}

func renderEnvelopeHead(sender, nonce, marker, body string) (string, error) {
	if !envelopeTokenPattern.MatchString(nonce) {
		return "", fmt.Errorf("envelope nonce %q is invalid", nonce)
	}
	return renderEnvelopeTag(envelopeHead(sender, nonce), marker, body)
}

// envelopeHead is the tag text shared by an envelope's opening and closing.
func envelopeHead(sender, nonce string) string {
	if sender == "" {
		return "gang#" + nonce
	}
	return "gang:" + sender + "#" + nonce
}

// envelopeOpening is the opening tag every envelope with this head and marker
// starts with. A body cannot repeat it: rendering defangs tag-shaped text.
func envelopeOpening(head, marker string) string {
	if marker != "" {
		return "[" + head + " " + marker + "]"
	}
	return "[" + head + "]"
}

func renderEnvelopeTag(head, marker, body string) (string, error) {
	if err := validateMessageText(body); err != nil {
		return "", err
	}
	if marker != "" && !envelopeMarkerPattern.MatchString(marker) {
		return "", fmt.Errorf("envelope marker %q is invalid", marker)
	}
	body = defangEnvelopeTags(body)
	if body != "" {
		body += " "
	}
	wire := fmt.Sprintf("%s %s[/%s]", envelopeOpening(head, marker), body, head)
	return wire, checkEnvelopeBudget(wire)
}

func checkEnvelopeBudget(wire string) error {
	encoded, err := json.Marshal(wire)
	if err != nil {
		return err
	}
	if len(encoded) > maximumMessageBytes {
		return refuseError("message exceeds the %d-byte encoded envelope budget; put details in a state file and send its path", maximumMessageBytes)
	}
	return nil
}

var clockPattern = regexp.MustCompile(`^([01][0-9]|2[0-3]):([0-5][0-9])$`)

// errScheduleForm states the forms parseSchedule accepts. Callers name the
// option or operand and the rejected value around it.
var errScheduleForm = errors.New("expected a positive DURATION such as 30m, HH:MM, or RFC3339")

func parseSchedule(value string, now time.Time) (time.Time, error) {
	if deadline, err := time.Parse(time.RFC3339, value); err == nil {
		return deadline, nil
	}
	if match := clockPattern.FindStringSubmatch(value); match != nil {
		hour, _ := strconv.Atoi(match[1])
		minute, _ := strconv.Atoi(match[2])
		result := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())
		if !result.After(now) {
			result = result.AddDate(0, 0, 1)
		}
		return result, nil
	}
	duration, err := parseDuration(value)
	if err != nil {
		return time.Time{}, errScheduleForm
	}
	return now.Add(duration), nil
}

func parseDuration(value string) (time.Duration, error) {
	if value == "" {
		return 0, fmt.Errorf("duration is empty")
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("invalid duration %q: %w", value, err)
	}
	if duration <= 0 {
		return 0, fmt.Errorf("duration must be positive")
	}
	return duration, nil
}
