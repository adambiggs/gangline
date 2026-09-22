package harness

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
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
	if event.Kind == "turn-failed" {
		return TurnFinished, event, nil
	}
	switch TurnBoundary(event.Kind) {
	case TurnStarted, TurnFinished, TurnCompactionStarted, TurnCompactionFinished:
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
// native submit hook observed. Claude Code wraps bracketed pastes in a
// pasted_content element before exposing them to UserPromptSubmit; the wrapper
// identifier is harness-owned, but the wrapped bytes remain authoritative.
func SubmittedPromptMatches(primitive Invocation, sent, witnessed string) (bool, error) {
	switch primitive.Name {
	case "exact-prompt":
		return witnessed == sent, nil
	case "claude-pasted-content":
		if witnessed == sent {
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
		return strings.TrimSuffix(wrapped, suffix) == sent, nil
	default:
		return false, fmt.Errorf("unknown submit-witness primitive %q", primitive.Name)
	}
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
