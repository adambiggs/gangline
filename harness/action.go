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

// SubmitPauses reports whether a message's submit key follows a fixed pause
// of the settle duration (before_submit: pause) rather than a stable read-back
// of the whole composer. A pause suits a harness whose submit hook confirms
// the exact message: a short pane can clip the composer or its footer, and
// then no read-back can succeed.
func SubmitPauses(invocation Invocation) (bool, error) {
	if invocation.Name != "enter-submit" {
		return false, fmt.Errorf("unknown submit primitive %q", invocation.Name)
	}
	switch invocation.Params["before_submit"] {
	case "":
		return false, nil
	case "pause":
		return true, nil
	default:
		return false, fmt.Errorf("unknown submit before_submit mode %q", invocation.Params["before_submit"])
	}
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

// BracketedPaste reports whether the submit primitive pastes text bracketed,
// so that no newline in it can submit the composer.
func BracketedPaste(invocation Invocation) bool {
	return invocation.Params["paste"] == "bracketed"
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
