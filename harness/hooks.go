package harness

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
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
