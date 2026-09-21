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
