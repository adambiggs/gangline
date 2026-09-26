package harness

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/adambiggs/gangline/substrate"
)

var (
	ErrNoComposer         = errors.New("no composer on screen")
	ErrComposerClipped    = errors.New("composer is clipped by the screen")
	ErrComposerOccupied   = errors.New("another surface owns input")
	ErrForeignComposer    = errors.New("composer belongs to a child session")
	ErrBackgroundComposer = errors.New("composer creates a background session")
)

type Composer struct {
	Text           string
	CollapsedChars int
	TailOccupied   bool
}

type StartupState string

const (
	StartupReady         StartupState = "ready"
	StartupTrustRequired StartupState = "trust-required"
	StartupOccupied      StartupState = "occupied"
)

type Startup struct {
	State  StartupState
	Prompt string
}

func InspectStartup(collar Collar, screen substrate.Screen) (Startup, error) {
	for _, invocation := range collar.Primitives.Startup {
		switch invocation.Name {
		case "claude-trust-prompt", "codex-trust-prompt":
			prompt, found := trustPrompt(invocation.Name, screen)
			if found {
				return Startup{State: StartupTrustRequired, Prompt: prompt}, nil
			}
		case "claude-composer", "codex-composer":
			if _, err := ReadComposer(invocation, screen); err == nil {
				return Startup{State: StartupReady}, nil
			} else if !errors.Is(err, ErrNoComposer) {
				if invocation.Name == "codex-composer" && errors.Is(err, ErrComposerOccupied) {
					blocked, found, detectErr := DetectBlocked(collar.Primitives.Blocked, screen)
					if detectErr != nil {
						return Startup{}, detectErr
					}
					if found {
						return Startup{State: StartupTrustRequired, Prompt: blocked.Evidence}, nil
					}
				}
				return Startup{State: StartupOccupied, Prompt: err.Error()}, nil
			}
		default:
			return Startup{}, fmt.Errorf("unknown startup primitive %q", invocation.Name)
		}
	}
	return Startup{State: StartupOccupied, Prompt: "no declared startup primitive recognized the screen"}, nil
}

func ReadComposer(invocation Invocation, screen substrate.Screen) (Composer, error) {
	switch invocation.Name {
	case "codex-composer":
		return readCodexComposer(screen)
	case "claude-composer":
		return readClaudeComposer(screen)
	default:
		return Composer{}, fmt.Errorf("unknown composer primitive %q", invocation.Name)
	}
}

var codexCollapsedPastePattern = regexp.MustCompile(`^\[Pasted Content ([1-9][0-9]*) chars\]$`)
var codexFooterPattern = regexp.MustCompile(`^[^\n]+ · Context [0-9]+% used$`)

// SameComposerText reports whether a composer read-back shows the submitted
// text. The composer wraps long input at the pane width and the reader joins
// the visual lines with newlines, so only the non-whitespace content is
// comparable.
func SameComposerText(composer, submitted string) bool {
	return strings.Join(strings.Fields(composer), "") == strings.Join(strings.Fields(submitted), "")
}

func readCodexComposer(screen substrate.Screen) (Composer, error) {
	lines := screenLines(screen, false)
	for index := len(lines) - 1; index >= 0; index-- {
		line := strings.TrimRight(lines[index], " \t")
		if !strings.HasPrefix(line, "›") {
			continue
		}
		if matched, _ := regexp.MatchString(`^› [0-9]+\. `, line); matched {
			return Composer{}, ErrComposerOccupied
		}
		text := strings.TrimPrefix(line, "›")
		text = strings.TrimPrefix(text, " ")
		text = strings.ReplaceAll(text, "\u00a0", "")
		composer := Composer{Text: text}
		blankBeforeFooter := false
		for _, following := range lines[index+1:] {
			following = strings.TrimSpace(following)
			if following == "" {
				blankBeforeFooter = true
				continue
			}
			if blankBeforeFooter && codexFooterPattern.MatchString(following) {
				break
			}
			composer.TailOccupied = true
			break
		}
		if match := codexCollapsedPastePattern.FindStringSubmatch(text); match != nil {
			if !composer.TailOccupied {
				composer.CollapsedChars, _ = strconv.Atoi(match[1])
			}
		}
		return composer, nil
	}
	return Composer{}, ErrNoComposer
}

func readClaudeComposer(screen substrate.Screen) (Composer, error) {
	lines := screenLines(screen, false)
	trimmed := make([]string, len(lines))
	for index, line := range lines {
		trimmed[index] = strings.TrimSpace(line)
	}

	if containsPair(trimmed, "Your conversation moved to the background", "describe a task for a new session") {
		return Composer{}, ErrBackgroundComposer
	}
	if hasClaudeOverlay(lines) {
		return Composer{}, ErrComposerOccupied
	}

	closing := lastRule(lines)
	if closing < 0 {
		return Composer{}, ErrNoComposer
	}
	prompt := firstNonblank(lines, closing+1, len(lines))
	if prompt >= 0 && strings.HasPrefix(lines[prompt], "❯") {
		return Composer{}, ErrComposerClipped
	}

	opening, named := previousRule(lines, closing)
	if opening < 0 {
		return Composer{}, ErrNoComposer
	}
	if named && !parentConversation(lines[closing+1:]) {
		return Composer{}, ErrForeignComposer
	}

	first := firstNonblank(lines, opening+1, closing)
	if first < 0 || !strings.HasPrefix(lines[first], "❯") {
		return Composer{}, ErrNoComposer
	}
	// Pasting a wrapped message expands the input frame. A tall frame needs
	// the active cursor inside its body to distinguish it from conversation
	// history; height alone does not identify a non-composer surface.
	if !named && closing-opening > 6 && (!screen.Cursor.Visible || screen.Cursor.Row < first || screen.Cursor.Row >= closing) {
		return Composer{}, ErrNoComposer
	}
	body := append([]string(nil), lines[first:closing]...)
	body[0] = strings.TrimPrefix(body[0], "❯")
	body[0] = strings.TrimPrefix(body[0], " ")
	for index := range body {
		body[index] = strings.ReplaceAll(body[index], "\u00a0", "")
	}
	if !named && len(body) == 1 && screen.Cursor.Visible && screen.Cursor.Row == first && screen.Cursor.Column == 2 && strings.HasPrefix(body[0], "Try \"") && strings.HasSuffix(body[0], "\"") {
		return Composer{}, nil
	}
	return Composer{Text: strings.TrimRight(strings.Join(body, "\n"), "\n")}, nil
}

func trustPrompt(name string, screen substrate.Screen) (string, bool) {
	flat := strings.Join(screenLines(screen, true), "\n")
	if name == "codex-trust-prompt" {
		title := regexp.MustCompile(`(?im)^[[:space:]]*(?:[0-9]+ )?hooks need review(?: before they can run)?[[:space:]]*$`)
		choice := regexp.MustCompile(`(?m)^[[:space:]]*[❯›>] (?:[0-9]+\. )?(?:Review hooks|Stop|PostCompact|UserPromptSubmit|PreCompact|PostToolUse|PermissionRequest)(?:[[:space:]]|$)`)
		if title.MatchString(flat) && choice.MatchString(flat) {
			return "native hooks need review before they can run", true
		}
	}
	selected := regexp.MustCompile(`(?m)^[[:space:]]*[❯›>] (?:[0-9]+\. )?(?:Yes|No|Trust|Allow|Continue)(?:[,[:space:]]|$)`).MatchString(flat)
	if !selected {
		return "", false
	}
	patterns := []string{"Do you trust the contents of this directory?"}
	if name == "claude-trust-prompt" {
		patterns = append(patterns, "Only use Claude Code with files you trust", "Important: Only use Claude Code with files you trust", "Quick safety check: Is this a project you created or one you trust?")
	}
	for _, pattern := range patterns {
		for _, line := range strings.Split(flat, "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), pattern) {
				return pattern, true
			}
		}
	}
	return "", false
}

func screenLines(screen substrate.Screen, includeDim bool) []string {
	lines := make([]string, len(screen.Rows))
	for rowIndex, row := range screen.Rows {
		var text strings.Builder
		for _, cell := range row {
			if !includeDim && cell.Attributes.Dim {
				continue
			}
			text.WriteString(cell.Text)
		}
		lines[rowIndex] = strings.TrimRight(text.String(), " \t")
	}
	return lines
}

func containsPair(lines []string, first, second string) bool {
	found := false
	for _, line := range lines {
		found = found || strings.Contains(line, first)
		if found && strings.Contains(line, second) {
			return true
		}
	}
	return false
}

func hasClaudeOverlay(lines []string) bool {
	band := -1
	for index, line := range lines {
		if onlyRune(strings.TrimSpace(line), '▔') {
			band = index
			continue
		}
		if band >= 0 && index > band+1 && strings.Contains(line, "Esc to ") {
			return true
		}
	}
	return false
}

func lastRule(lines []string) int {
	for index := len(lines) - 1; index >= 0; index-- {
		if onlyRune(strings.TrimSpace(lines[index]), '─') {
			return index
		}
	}
	return -1
}

func previousRule(lines []string, before int) (int, bool) {
	closingWidth := len([]rune(strings.TrimSpace(lines[before])))
	for index := before - 1; index >= 0; index-- {
		line := strings.TrimSpace(lines[index])
		if onlyRune(line, '─') && len([]rune(line)) == closingWidth {
			return index, false
		}
		if strings.Count(line, "─") > 10 && strings.HasPrefix(line, "─") && strings.HasSuffix(line, "─") {
			return index, true
		}
	}
	return -1, false
}

func onlyRune(text string, want rune) bool {
	if text == "" {
		return false
	}
	for _, char := range text {
		if char != want {
			return false
		}
	}
	return true
}

func firstNonblank(lines []string, start, end int) int {
	for index := start; index < end; index++ {
		if strings.TrimSpace(lines[index]) != "" {
			return index
		}
	}
	return -1
}

func parentConversation(lines []string) bool {
	mode := false
	main := true
	for _, line := range lines {
		mode = mode || strings.Contains(line, " mode on")
		line = strings.TrimSpace(strings.TrimPrefix(line, "❯"))
		if strings.HasPrefix(line, "● ") {
			name := strings.Fields(strings.TrimPrefix(line, "● "))
			main = len(name) != 0 && name[0] == "main"
		}
	}
	return mode && main
}

var contextPattern = regexp.MustCompile(`(?:ctx )?([0-9]+(?:\.[0-9]+)?)([kKmM]?)/([0-9]+(?:\.[0-9]+)?)([kKmM]?)\s+([0-9]+)%`)

type ContextReading struct {
	Used    int64
	Limit   int64
	Percent float64
}

func ReadContext(invocation Invocation, screen substrate.Screen) (ContextReading, error) {
	switch invocation.Name {
	case "claude-screen-context", "codex-screen-context":
	default:
		return ContextReading{}, fmt.Errorf("unknown context primitive %q", invocation.Name)
	}
	flat := strings.Join(screenLines(screen, true), "\n")
	matches := contextPattern.FindAllStringSubmatch(flat, -1)
	if len(matches) == 0 {
		return ContextReading{}, errors.New("screen carries no context reading")
	}
	match := matches[len(matches)-1]
	used, err := scaledInteger(match[1], match[2])
	if err != nil {
		return ContextReading{}, err
	}
	limit, err := scaledInteger(match[3], match[4])
	if err != nil || limit == 0 {
		return ContextReading{}, errors.New("screen context limit is invalid")
	}
	percent, err := strconv.ParseFloat(match[5], 64)
	if err != nil || percent < 0 || percent > 100 {
		return ContextReading{}, errors.New("screen context percentage is invalid")
	}
	return ContextReading{Used: used, Limit: limit, Percent: percent / 100}, nil
}

func scaledInteger(number, suffix string) (int64, error) {
	value, err := strconv.ParseFloat(number, 64)
	if err != nil {
		return 0, errors.New("screen context count is invalid")
	}
	scale := float64(1)
	switch strings.ToLower(suffix) {
	case "k":
		scale = 1_000
	case "m":
		scale = 1_000_000
	}
	return int64(value * scale), nil
}
