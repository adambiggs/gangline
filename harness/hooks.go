package harness

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

type HookEvent struct {
	NativeEvent string
	Kind        string
	Payload     map[string]string
}

type TurnBoundary string

const (
	TurnStarted            TurnBoundary = "turn-started"
	TurnFinished           TurnBoundary = "turn-finished"
	TurnFailed             TurnBoundary = "turn-failed"
	TurnCompactionStarted  TurnBoundary = "compaction-started"
	TurnCompactionFinished TurnBoundary = "compaction-finished"
)

// OpenTurnQuiet is how long an idle-looking screen must stay unchanged before
// a submitted turn with no finish boundary counts as over, as after Escape or
// a turn that dies without its Stop hook. Zero leaves turn state to the screen.
func OpenTurnQuiet(invocation Invocation) (time.Duration, error) {
	if invocation.Name != "hook-boundary" {
		return 0, fmt.Errorf("unknown turn-boundary primitive %q", invocation.Name)
	}
	value := invocation.Params["open_turn_quiet"]
	if value == "" {
		return 0, nil
	}
	duration, err := time.ParseDuration(value)
	if err != nil || duration <= 0 {
		return 0, fmt.Errorf("turn boundary open_turn_quiet %q is not a positive duration", value)
	}
	return duration, nil
}

func DetectTurnBoundary(collar Collar, data []byte) (TurnBoundary, HookEvent, error) {
	if collar.Primitives.TurnBoundary.Name != "hook-boundary" {
		return "", HookEvent{}, fmt.Errorf("unknown turn-boundary primitive %q", collar.Primitives.TurnBoundary.Name)
	}
	event, err := DecodeHook(collar, data)
	if err != nil {
		return "", HookEvent{}, err
	}
	switch TurnBoundary(event.Kind) {
	case TurnStarted, TurnFinished, TurnFailed, TurnCompactionStarted, TurnCompactionFinished:
		return TurnBoundary(event.Kind), event, nil
	default:
		return "", event, nil
	}
}

func DecodeHook(collar Collar, data []byte) (HookEvent, error) {
	if collar.Hooks == nil {
		return HookEvent{}, fmt.Errorf("collar %q declares no hooks", collar.Name)
	}
	var body map[string]any
	if err := json.Unmarshal(data, &body); err != nil {
		return HookEvent{}, fmt.Errorf("decode hook payload: %w", err)
	}
	native, ok := stringField(body, "hook_event_name")
	if !ok {
		native, ok = stringField(body, "event")
	}
	if !ok || native == "" {
		return HookEvent{}, errors.New("hook payload carries no event name")
	}
	wiring, ok := collar.Hooks.Events[normalizeEventName(native)]
	if !ok {
		return HookEvent{}, fmt.Errorf("collar %q does not map native hook event %q", collar.Name, native)
	}
	payload := make(map[string]string, len(wiring.Payload))
	for target, source := range wiring.Payload {
		if value, ok := nestedString(body, strings.Split(source, ".")); ok {
			payload[target] = value
		}
	}
	return HookEvent{NativeEvent: native, Kind: wiring.Event, Payload: payload}, nil
}

// SubmittedPromptMatches compares the text Gangline sent with the prompt a
// native submit hook observed. Claude Code wraps a long bracketed paste in a
// pasted_content element before exposing it to UserPromptSubmit; the wrapper
// identifier is harness-owned, but the pasted bytes remain authoritative,
// wrapped or not, apart from the escape described at consumePastedBody.
func SubmittedPromptMatches(primitive Invocation, sent, witnessed string) (bool, error) {
	switch primitive.Name {
	case "exact-prompt":
		return witnessed == sent, nil
	case "claude-pasted-content":
		if rest, ok := consumePastedBody(witnessed, sent); ok && rest == "" {
			return true, nil
		}
		framed := witnessed
		if strings.HasPrefix(framed, "\n\n") {
			framed = strings.TrimPrefix(framed, "\n\n")
		} else {
			framed = strings.TrimPrefix(framed, "\n")
		}
		const prefix = "<pasted_content id=\""
		if !strings.HasPrefix(framed, prefix) {
			return false, nil
		}
		identifier, wrapped, found := strings.Cut(strings.TrimPrefix(framed, prefix), "\">\n")
		if !found || identifier == "" || strings.ContainsAny(identifier, "\"\r\n<>") {
			return false, nil
		}
		suffix := "\n</pasted_content id=\"" + identifier + "\">"
		wrapped = strings.TrimSuffix(wrapped, "\n")
		if !strings.HasSuffix(wrapped, suffix) {
			return false, nil
		}
		rest, ok := consumePastedBody(strings.TrimSuffix(wrapped, suffix), sent)
		return ok && rest == "", nil
	default:
		return false, fmt.Errorf("unknown submit-witness primitive %q", primitive.Name)
	}
}

// pastePlaceholder is the reference pattern Claude Code expands in a
// submitted prompt: a bracketed label for pasted text, an image, or truncated
// text, a paste number, an optional line count, and any trailing dots. Claude
// Code substitutes any such token whose number names a live text paste,
// whatever its label.
var pastePlaceholder = regexp.MustCompile(`\[(?:Pasted text|Image|\.\.\.Truncated text) #\d+(?: \+\d+ lines)?\.*\]`)

// PasteHazard returns why text cannot be typed into a harness unchanged.
// Claude Code replaces a placeholder token with the content of a live earlier
// paste carrying that number, and a paste stays live after its
// composer is cleared. Gangline cannot see which numbers are live, so every
// such token is refused.
func PasteHazard(primitive Invocation, text string) (string, error) {
	switch primitive.Name {
	case "exact-prompt":
		return "", nil
	case "claude-pasted-content":
		if pastePlaceholder.MatchString(text) {
			return "text contains a numbered paste placeholder in brackets, for pasted text, an image, or truncated text; Claude Code can replace it with the content of an earlier paste, so describe it in words", nil
		}
		return "", nil
	default:
		return "", fmt.Errorf("unknown submit-witness primitive %q", primitive.Name)
	}
}

// SubmittedPromptStartsWith accepts a continuation followed by native-merged
// steers. A newline is the boundary Codex inserts between submitted messages.
func SubmittedPromptStartsWith(primitive Invocation, sent, witnessed string) (bool, error) {
	matched, err := SubmittedPromptMatches(primitive, sent, witnessed)
	if err != nil || matched {
		return matched, err
	}
	switch primitive.Name {
	case "exact-prompt":
		return strings.HasPrefix(witnessed, sent+"\n"), nil
	case "claude-pasted-content":
		framed := witnessed
		if strings.HasPrefix(framed, "\n\n") {
			framed = strings.TrimPrefix(framed, "\n\n")
		} else {
			framed = strings.TrimPrefix(framed, "\n")
		}
		const prefix = "<pasted_content id=\""
		if !strings.HasPrefix(framed, prefix) {
			return false, nil
		}
		identifier, wrapped, found := strings.Cut(strings.TrimPrefix(framed, prefix), "\">\n")
		if !found || identifier == "" || strings.ContainsAny(identifier, "\"\r\n<>") {
			return false, nil
		}
		suffix := "\n</pasted_content id=\"" + identifier + "\">"
		rest, ok := consumePastedBody(wrapped, sent)
		return ok && strings.HasPrefix(rest, suffix+"\n"), nil
	default:
		return false, fmt.Errorf("unknown submit-witness primitive %q", primitive.Name)
	}
}

// consumePastedBody matches sent against the start of pasted text as Claude
// Code reports it and returns the rest. Claude Code escapes pasted text that
// could read as its own wrapper tag, whether or not it wraps the paste: it
// replaces the tag's opener, an ASCII '<' or a non-ASCII lookalike, with an
// ASCII `<\`. That pair may stand for one such sent opener; every other
// byte must match.
func consumePastedBody(body, sent string) (string, bool) {
	for sent != "" {
		r, size := utf8.DecodeRuneInString(sent)
		if strings.HasPrefix(body, `<\`) && !strings.HasPrefix(sent, `<\`) && (r == '<' || r >= utf8.RuneSelf) {
			body, sent = body[2:], sent[size:]
			continue
		}
		if !strings.HasPrefix(body, sent[:size]) {
			return "", false
		}
		body, sent = body[size:], sent[size:]
	}
	return body, true
}

func normalizeEventName(name string) string {
	return strings.Map(func(char rune) rune {
		if char == '-' || char == '_' || char == ' ' {
			return -1
		}
		return char
	}, strings.ToLower(name))
}

func stringField(body map[string]any, field string) (string, bool) {
	value, ok := body[field].(string)
	return value, ok
}

func nestedString(value any, path []string) (string, bool) {
	for len(path) > 1 {
		object, ok := value.(map[string]any)
		if !ok {
			return "", false
		}
		value, ok = object[path[0]]
		if !ok {
			return "", false
		}
		path = path[1:]
	}
	object, ok := value.(map[string]any)
	if !ok || len(path) == 0 {
		return "", false
	}
	result, ok := object[path[0]].(string)
	return result, ok
}
