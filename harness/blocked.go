package harness

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/adambiggs/gangline/substrate"
)

type Blocked struct {
	Evidence string
}

func detectBlocked(invocation Invocation, lines []string) (Blocked, bool, error) {
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
	text := strings.Join(lines, "\n")
	if !prompt.MatchString(text) || !choice.MatchString(text) {
		return Blocked{}, false, nil
	}
	return Blocked{Evidence: fmt.Sprintf("native input choice: %s; %s", strings.Join(strings.Fields(prompt.FindString(text)), " "), strings.Join(strings.Fields(choice.FindString(text)), " "))}, true, nil
}

// nativeInputLines returns the screen lines where a native prompt can be
// drawn, or owned when the agent's own composer holds input. Prompt text
// elsewhere on screen belongs to a reply, a diff, or a draft.
func nativeInputLines(composer Invocation, screen substrate.Screen) ([]string, bool, error) {
	lines := screenLines(screen, true)
	switch composer.Name {
	case "codex-composer":
		// Codex draws a native prompt in place of its composer. A list can
		// mark its selected row with the composer's ›, so a readable composer
		// owns input only while it holds the cursor.
		composer, err := readCodexComposer(screen)
		if err == nil && composer.holdsCursor(screen) {
			return nil, true, nil
		}
		return codexInputLines(lines), false, nil
	case "claude-composer":
		// Claude Code draws a native prompt below the last rule line, or over
		// the whole screen when the prompt leaves no rule visible. A readable
		// composer rules a prompt out only while it holds the cursor: a rule
		// line in the conversation can open a composer frame that the prompt's
		// own border closes.
		composer, err := readClaudeComposer(screen)
		if errors.Is(err, ErrComposerClipped) || err == nil && composer.holdsCursor(screen) {
			return nil, true, nil
		}
		rule := lastRule(screenLines(screen, false))
		if rule < 0 {
			// A full-screen view such as the transcript draws its only rule dim.
			rule = lastRule(lines)
		}
		if rule >= 0 {
			lines = lines[rule+1:]
		}
		return lines, false, nil
	default:
		return nil, false, fmt.Errorf("unknown composer primitive %q", composer.Name)
	}
}

// codexInputLines returns the lines below the conversation. Codex heads each
// agent history cell with • at the first column and draws a prompt or picker
// after two blank lines, while one blank line separates paragraphs within a
// cell. The region starts after the first two blank lines below the last cell
// head; with no cell head or no such lines, every line can hold a prompt.
func codexInputLines(lines []string) []string {
	head := -1
	for index, line := range lines {
		if strings.HasPrefix(line, "• ") {
			head = index
		}
	}
	if head < 0 {
		return lines
	}
	for index := head + 1; index+1 < len(lines); index++ {
		if strings.TrimSpace(lines[index]) == "" && strings.TrimSpace(lines[index+1]) == "" {
			return lines[index+2:]
		}
	}
	return lines
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
	return Blocked{}, false, nil
}
