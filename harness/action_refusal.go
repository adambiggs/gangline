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

// CompactionActive reports whether the screen shows the harness compacting.
// A collar that declares no active pattern never reports one.
func CompactionActive(c Collar, screen substrate.Screen) (bool, error) {
	if c.Actions.Compact.Active == "" {
		return false, nil
	}
	pattern, err := regexp.Compile(c.Actions.Compact.Active)
	if err != nil {
		return false, err
	}
	return pattern.MatchString(strings.Join(screenLines(screen, true), "\n")), nil
}
