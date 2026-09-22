package harness

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/adambiggs/gangline/substrate"
)

type Blocked struct {
	Evidence string
}

func DetectBlocked(invocation Invocation, screen substrate.Screen) (Blocked, bool, error) {
	if invocation.Name != "screen-blocked" {
		return Blocked{}, false, fmt.Errorf("unknown blocked primitive %q", invocation.Name)
	}
	prompt, err := blockedPattern(invocation, "prompt")
	if err != nil {
		return Blocked{}, false, err
	}
	choice, err := blockedPattern(invocation, "choice")
	if err != nil {
		return Blocked{}, false, err
	}
	text := strings.Join(screenLines(screen, true), "\n")
	if !prompt.MatchString(text) || !choice.MatchString(text) {
		return Blocked{}, false, nil
	}
	return Blocked{Evidence: "runtime approval surface matched collar prompt and choice rules"}, true, nil
}

func blockedPattern(invocation Invocation, name string) (*regexp.Regexp, error) {
	expression := invocation.Params[name]
	if expression == "" {
		return nil, fmt.Errorf("blocked primitive has no %s pattern", name)
	}
	pattern, err := regexp.Compile(expression)
	if err != nil {
		return nil, fmt.Errorf("blocked primitive %s pattern: %w", name, err)
	}
	return pattern, nil
}

// InputBlocked includes launch trust that can appear after the first composer.
func InputBlocked(collar Collar, screen substrate.Screen) (Blocked, bool, error) {
	startup, err := InspectStartup(collar, screen)
	if err != nil {
		return Blocked{}, false, err
	}
	if startup.State == StartupTrustRequired {
		return Blocked{Evidence: startup.Prompt}, true, nil
	}
	return DetectBlocked(collar.Primitives.Blocked, screen)
}
