package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
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
	if !utf8.ValidString(body) || strings.IndexByte(body, 0) >= 0 {
		return "", refuseError("message body must be UTF-8 text without NUL bytes")
	}
	return body, nil
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
var tagShapedTextPattern = regexp.MustCompile(`(?i)[\[［〔【]([[:space:]]*/?[[:space:]]*gang[[:space:]]*:)`)

func renderEnvelope(sender, nonce, marker, body string) (string, error) {
	if !envelopeTokenPattern.MatchString(sender) {
		return "", fmt.Errorf("envelope sender %q is invalid", sender)
	}
	if !envelopeTokenPattern.MatchString(nonce) {
		return "", fmt.Errorf("envelope nonce %q is invalid", nonce)
	}
	return renderEnvelopeTag(sender+"#"+nonce, marker, body)
}

func renderEnvelopeTag(tag, marker, body string) (string, error) {
	if marker != "" && !envelopeMarkerPattern.MatchString(marker) {
		return "", fmt.Errorf("envelope marker %q is invalid", marker)
	}
	suffix := ""
	if marker != "" {
		suffix = " " + marker
	}
	body = tagShapedTextPattern.ReplaceAllString(body, "$1")
	wire := fmt.Sprintf("[gang:%s%s] %s [/gang:%s]", tag, suffix, body, tag)
	encoded, err := json.Marshal(wire)
	if err != nil {
		return "", err
	}
	if len(encoded) > maximumMessageBytes {
		return "", refuseError("message exceeds the %d-byte encoded envelope budget; put details in a state file and send its path", maximumMessageBytes)
	}
	return wire, nil
}

var durationPartPattern = regexp.MustCompile(`([0-9]+)([hms])`)
var clockPattern = regexp.MustCompile(`^([01][0-9]|2[0-3]):([0-5][0-9])$`)

func parseSchedule(value string, now time.Time) (time.Time, error) {
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
		return time.Time{}, err
	}
	return now.Add(duration), nil
}

func parseDuration(value string) (time.Duration, error) {
	if value == "" {
		return 0, fmt.Errorf("duration is empty")
	}
	remaining := value
	var duration time.Duration
	for remaining != "" {
		match := durationPartPattern.FindStringSubmatchIndex(remaining)
		if match == nil || match[0] != 0 {
			return 0, fmt.Errorf("invalid duration %q; use values such as 2h30m or 45s", value)
		}
		amount, err := strconv.ParseUint(remaining[match[2]:match[3]], 10, 63)
		if err != nil {
			return 0, fmt.Errorf("invalid duration %q", value)
		}
		var unit time.Duration
		switch remaining[match[4]:match[5]] {
		case "h":
			unit = time.Hour
		case "m":
			unit = time.Minute
		case "s":
			unit = time.Second
		}
		if amount > uint64((time.Duration(1<<63-1)-duration)/unit) {
			return 0, fmt.Errorf("duration %q is too large", value)
		}
		duration += time.Duration(amount) * unit
		remaining = remaining[match[1]:]
	}
	if duration <= 0 {
		return 0, fmt.Errorf("duration must be positive")
	}
	return duration, nil
}
