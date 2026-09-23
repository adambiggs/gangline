package harness

import (
	"regexp"
	"strings"

	"github.com/adambiggs/gangline/substrate"
)

// Count matches so an unchanged historical refusal cannot fail a new action.
func ActionRefusals(action Action, screen substrate.Screen) ([]string, error) {
	if action.Refusal == "" {
		return nil, nil
	}
	pattern, err := regexp.Compile(action.Refusal)
	if err != nil {
		return nil, err
	}
	return pattern.FindAllString(strings.Join(screenLines(screen, true), "\n"), -1), nil
}
