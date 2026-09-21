package harness

import (
	"fmt"
	"time"

	"github.com/adambiggs/gangline/substrate"
)

func Submit(invocation Invocation, text string) (Action, error) {
	if invocation.Name != "enter-submit" {
		return Action{}, fmt.Errorf("unknown submit primitive %q", invocation.Name)
	}
	return Action{Text: text, Submit: true}, nil
}

func SubmitSettle(invocation Invocation) (time.Duration, error) {
	if invocation.Name != "enter-submit" {
		return 0, fmt.Errorf("unknown submit primitive %q", invocation.Name)
	}
	value := invocation.Params["settle"]
	if value == "" {
		return 0, nil
	}
	duration, err := time.ParseDuration(value)
	if err != nil || duration < 0 {
		return 0, fmt.Errorf("submit settle %q is not a non-negative duration", value)
	}
	return duration, nil
}

func SubmitInput(invocation Invocation, text string) (substrate.Keys, error) {
	if invocation.Name != "enter-submit" {
		return substrate.Keys{}, fmt.Errorf("unknown submit primitive %q", invocation.Name)
	}
	switch invocation.Params["paste"] {
	case "":
		return substrate.Keys{Text: text}, nil
	case "bracketed":
		return substrate.Keys{Text: "\x1b[200~" + text + "\x1b[201~"}, nil
	default:
		return substrate.Keys{}, fmt.Errorf("unknown submit paste mode %q", invocation.Params["paste"])
	}
}

func RenderAction(action Action, values map[string]string) (Action, error) {
	text, err := renderArgs([]string{action.Text}, values)
	if err != nil {
		return Action{}, err
	}
	return Action{Text: text[0], Keys: append([]string(nil), action.Keys...), Submit: action.Submit}, nil
}

func (action Action) Input() substrate.Keys {
	return substrate.Keys{Text: action.Text, Names: append([]string(nil), action.Keys...), Submit: action.Submit}
}
