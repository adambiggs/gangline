package harness

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/adambiggs/gangline/substrate"
)

type Capacity struct {
	Fingerprint string
	Evidence    string
}

var ErrCapacityIdentity = errors.New("terminal capacity failure identity is unknown")

type CapacityTurn struct {
	SessionID, TurnID, Failure, Prompt string
}

func (turn CapacityTurn) Fingerprint() string {
	if turn.SessionID == "" || turn.TurnID == "" {
		return ""
	}
	digest := sha256.Sum256([]byte("native\x00" + turn.SessionID + "\x00" + turn.TurnID))
	return fmt.Sprintf("%x", digest)
}

// DetectCapacity requires the final native history item and its visible user
// prompt, not an error anywhere in scrollback. A clipped prompt cannot identify
// a fresh failure, so it is unknown rather than permission to repeat input.
func DetectCapacity(collar Collar, screen substrate.Screen) (Capacity, bool, error) {
	return DetectCapacityForTurn(collar, screen, CapacityTurn{})
}

// IsCapacityFailure matches only the failure declared by this collar. Native
// error classes survive changes to the provider's explanatory text.
func IsCapacityFailure(collar Collar, reason string) bool {
	p := collar.Primitives.Capacity
	if p == nil || reason == "" {
		return false
	}
	class := p.Params["error_class"]
	return reason == p.Params["error"] || class != "" && (reason == class || strings.HasPrefix(reason, class+": "))
}

// DetectCapacityForTurn uses an attributed native failure when a turn identity
// is available. A newer or unfinished turn must not inherit an old screen
// error. Legacy clients without turn identity retain the visible-prompt check.
func DetectCapacityForTurn(collar Collar, screen substrate.Screen, turn CapacityTurn) (Capacity, bool, error) {
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
	if turn.TurnID != "" && turn.Failure != "" {
		if turn.SessionID == "" || !IsCapacityFailure(collar, turn.Failure) {
			return Capacity{}, false, nil
		}
		return Capacity{Fingerprint: turn.Fingerprint(), Evidence: turn.Failure}, true, nil
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
		return Capacity{}, false, fmt.Errorf("%w: no visible originating prompt", ErrCapacityIdentity)
	}
	identity := strings.Join(strings.Fields(strings.Join(lines[prompt:errorStart], " ")), " ")
	if identity == "›" || strings.Contains(identity, "[Pasted Content") || (strings.Contains(identity, "[gang:") && !strings.Contains(identity, "[/gang:")) {
		return Capacity{}, false, fmt.Errorf("%w: originating prompt is clipped or collapsed", ErrCapacityIdentity)
	}
	if turn.TurnID != "" {
		// A one-time envelope binds this visible history to the current
		// submit witness. Repeated keyboard text cannot distinguish turns.
		prompt := strings.Join(strings.Fields(turn.Prompt), " ")
		opener := regexp.MustCompile(`\[gang:([^\]# ]+)#([0-9a-f]{16})( [^\]]+)?\]`).FindStringSubmatch(prompt)
		if turn.SessionID == "" || len(opener) == 0 || !strings.Contains(prompt, "[/gang:"+opener[1]+"#"+opener[2]+"]") || !strings.HasPrefix(identity, "› "+prompt) {
			return Capacity{}, false, fmt.Errorf("%w: visible prompt does not identify the current submitted envelope", ErrCapacityIdentity)
		}
		return Capacity{Fingerprint: turn.Fingerprint(), Evidence: errorText}, true, nil
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
