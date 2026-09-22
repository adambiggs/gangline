package harness

import (
	"crypto/sha256"
	"fmt"
	"regexp"
	"strings"

	"github.com/adambiggs/gangline/substrate"
)

type Capacity struct {
	Fingerprint string
	Evidence    string
}

// DetectCapacity requires the final native history item and its visible user
// prompt, not an error anywhere in scrollback. A clipped prompt cannot identify
// a fresh failure, so it is unknown rather than permission to repeat input.
func DetectCapacity(collar Collar, screen substrate.Screen) (Capacity, bool, error) {
	primitive := collar.Primitives.Capacity
	if primitive == nil {
		return Capacity{}, false, nil
	}
	if err := validateCapacity(*primitive); err != nil {
		return Capacity{}, false, err
	}
	if _, blocked, err := InputBlocked(collar, screen); err != nil || blocked {
		return Capacity{}, false, err
	}
	idle, err := Idle(collar, screen)
	if err != nil || !idle {
		return Capacity{}, false, err
	}
	lines := screenLines(screen, true)
	retrying := regexp.MustCompile(primitive.Params["retrying"])
	composer := -1
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.HasPrefix(lines[i], "›") {
			composer = i
			break
		}
	}
	if composer < 0 {
		return Capacity{}, false, ErrNoComposer
	}
	// Retry status below the current composer is native chrome. Text in an
	// earlier turn or in the user's prompt cannot suppress a terminal error.
	if retrying.MatchString(strings.Join(lines[composer+1:], "\n")) {
		return Capacity{}, false, nil
	}

	last := composer - 1
	for last >= 0 && strings.TrimSpace(lines[last]) == "" {
		last--
	}
	errorStart := last
	for errorStart >= 0 && !strings.HasPrefix(lines[errorStart], "■ ") {
		if strings.TrimSpace(lines[errorStart]) == "" {
			return Capacity{}, false, nil
		}
		errorStart--
	}
	if errorStart < 0 {
		return Capacity{}, false, nil
	}
	errorText := strings.Join(strings.Fields(strings.Join(lines[errorStart:last+1], " ")), " ")
	if errorText != "■ "+primitive.Params["error"] {
		return Capacity{}, false, nil
	}
	prompt := errorStart - 1
	for prompt >= 0 && !strings.HasPrefix(lines[prompt], "› ") {
		prompt--
	}
	if prompt < 0 {
		return Capacity{}, false, fmt.Errorf("terminal capacity error has no visible originating prompt; failure identity is unknown")
	}
	identity := strings.Join(strings.Fields(strings.Join(lines[prompt:errorStart], " ")), " ")
	if identity == "›" || strings.Contains(identity, "[Pasted Content") || (strings.Contains(identity, "[gang:") && !strings.Contains(identity, "[/gang:")) {
		return Capacity{}, false, fmt.Errorf("terminal capacity originating prompt is clipped or collapsed; failure identity is unknown")
	}
	digest := sha256.Sum256([]byte(identity + "\n" + errorText))
	return Capacity{Fingerprint: fmt.Sprintf("%x", digest), Evidence: errorText}, true, nil
}

func validateCapacity(primitive Invocation) error {
	if primitive.Name != "codex-terminal-capacity" {
		return fmt.Errorf("unknown capacity primitive %q", primitive.Name)
	}
	if primitive.Params["error"] == "" || primitive.Params["retrying"] == "" {
		return fmt.Errorf("capacity primitive requires error and retrying declarations")
	}
	if _, err := regexp.Compile(primitive.Params["retrying"]); err != nil {
		return fmt.Errorf("capacity retrying expression: %w", err)
	}
	return nil
}
