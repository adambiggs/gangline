package harness

import (
	"fmt"

	"github.com/adambiggs/gangline/substrate"
)

func Submit(invocation Invocation, text string) (Action, error) {
	if invocation.Name != "enter-submit" {
		return Action{}, fmt.Errorf("unknown submit primitive %q", invocation.Name)
	}
	return Action{Text: text, Submit: true}, nil
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
