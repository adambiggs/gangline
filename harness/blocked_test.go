package harness

import (
	"strings"
	"testing"

	"github.com/adambiggs/gangline/substrate"
)

func TestEmbeddedCollarsDetectRuntimeApprovalSurfaces(t *testing.T) {
	tests := []struct {
		name   string
		collar string
		file   string
		lines  []string
	}{
		{name: "codex hook review", collar: "codex", lines: []string{"Hooks need review", "› 1. Review hooks", "2. Trust all and continue", "Press enter to confirm or esc to go back"}},
		{name: "codex command approval", collar: "codex", file: "codex-0.151.0-command-approval.txt"},
		{name: "claude bash permission", collar: "claude", file: "claude-code-2.1.287-permission-bash.txt"},
		{name: "claude permission taller than the screen", collar: "claude", file: "claude-code-2.1.287-permission-tall.txt"},
		{name: "claude question", collar: "claude", file: "claude-code-2.1.287-ask-user-question.txt"},
		{name: "claude multi-select question", collar: "claude", file: "claude-code-2.1.287-ask-user-question-multiselect.txt"},
		{name: "claude question in tabs", collar: "claude", file: "claude-code-2.1.287-ask-user-question-tabs.txt"},
		{name: "claude question submit tab", collar: "claude", file: "claude-code-2.1.287-ask-user-question-submit.txt"},
		{name: "claude question taller than the screen", collar: "claude", file: "claude-code-2.1.287-ask-user-question-tall.txt"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			collar, err := EmbeddedCollar(test.collar)
			if err != nil {
				t.Fatal(err)
			}
			screen := linesScreen(test.lines)
			if test.file != "" {
				screen = fixtureScreen(t, test.file)
			}
			blocked, found, err := InputBlocked(collar, screen)
			if err != nil || !found || blocked.Evidence == "" {
				t.Fatalf("blocked = %#v, found = %t, err = %v", blocked, found, err)
			}
		})
	}
}

// The collar's own prompt and choice patterns quote text that they match, so
// an agent that shows them in a reply, a diff, or a draft displays a screen
// that both patterns match without any native prompt on it.
const blockedPatternText = `prompt: "Do you want to proceed\\?|Allow .*\\?" choice: "1\\. Yes|Allow"`

func TestBlockedIgnoresPatternTextOutsideTheInputSurface(t *testing.T) {
	claudeReply := fixtureLines(t, "claude-code-2.1.287-pattern-in-reply.txt")
	tests := []struct {
		name   string
		collar string
		lines  []string
	}{
		{name: "claude reply above an empty composer", collar: "claude", lines: claudeReply},
		{name: "claude draft in the composer", collar: "claude", lines: []string{
			"────────", "❯ " + blockedPatternText, "────────", "  48k/200k (24%)",
		}},
		{name: "claude draft clipped by the screen bottom", collar: "claude", lines: []string{
			"● reply", "────────", "❯ " + blockedPatternText, "  Do you want to proceed?", "  1. Yes",
		}},
		{name: "claude reply above a child session composer", collar: "claude", lines: append([]string{blockedPatternText},
			fixtureLines(t, "claude-selected-subagent.txt")...)},
		{name: "claude reply above background sessions", collar: "claude", lines: append([]string{blockedPatternText},
			fixtureLines(t, "claude-background-sessions.txt")...)},
		{name: "codex reply above the composer", collar: "codex", lines: append([]string{
			"• Would you like to run the following command?", "  Yes, proceed",
		}, fixtureLines(t, "codex-0.151.0-composer.txt")...)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			collar, err := EmbeddedCollar(test.collar)
			if err != nil {
				t.Fatal(err)
			}
			screen := linesScreen(test.lines)
			if test.collar == "codex" {
				screen = codexComposerScreen(test.lines)
			}
			blocked, found, err := InputBlocked(collar, screen)
			if err != nil || found {
				t.Fatalf("blocked = %#v, found = %t, err = %v", blocked, found, err)
			}
		})
	}
}

// The transcript view replaces the composer with a footer under a dim rule,
// so no rule is visible to a reader that skips dim cells.
func TestBlockedIgnoresPatternTextAboveADimRule(t *testing.T) {
	collar, err := EmbeddedCollar("claude")
	if err != nil {
		t.Fatal(err)
	}
	lines := fixtureLines(t, "claude-code-2.1.287-transcript-view.txt")
	rows := make([][]substrate.Cell, len(lines))
	for index, line := range lines {
		rows[index] = testCells(line, onlyRune(line, '─'))
	}
	blocked, found, err := InputBlocked(collar, testScreen(rows...))
	if err != nil || found {
		t.Fatalf("blocked = %#v, found = %t, err = %v", blocked, found, err)
	}
}

// Claude Code lists command suggestions below the composer while the user
// types a slash command, and a description can carry prompt and choice text.
// The active cursor in the composer shows that the composer holds input.
func TestBlockedIgnoresACommandSuggestionBelowTheComposer(t *testing.T) {
	collar, err := EmbeddedCollar("claude")
	if err != nil {
		t.Fatal(err)
	}
	screen := fixtureScreen(t, "claude-code-2.1.287-command-suggestion.txt")
	screen.Cursor = substrate.Cursor{Row: 5, Column: 6, Visible: true}
	blocked, found, err := InputBlocked(collar, screen)
	if err != nil || found {
		t.Fatalf("blocked = %#v, found = %t, err = %v", blocked, found, err)
	}
}

// Codex keeps the cursor on the last line of a multi-line draft, below the
// composer's › row.
func TestBlockedIgnoresPatternTextInACodexDraft(t *testing.T) {
	collar, err := EmbeddedCollar("codex")
	if err != nil {
		t.Fatal(err)
	}
	screen := fixtureScreen(t, "codex-0.160.0-draft.txt")
	screen.Cursor = substrate.Cursor{Row: 20, Column: 2, Visible: true}
	blocked, found, err := InputBlocked(collar, screen)
	if err != nil || found {
		t.Fatalf("blocked = %#v, found = %t, err = %v", blocked, found, err)
	}
}

// Codex draws a numbered picker in place of its composer, below a reply that
// quotes prompt and choice text. One blank line separates paragraphs within
// a reply.
func TestBlockedIgnoresPatternTextAboveACodexPicker(t *testing.T) {
	collar, err := EmbeddedCollar("codex")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{"codex-0.160.0-model-picker.txt", "codex-0.160.0-model-picker-paragraphs.txt"} {
		t.Run(file, func(t *testing.T) {
			screen := fixtureScreen(t, file)
			screen.Cursor = substrate.Cursor{Row: 23, Column: 80}
			blocked, found, err := InputBlocked(collar, screen)
			if err != nil || found {
				t.Fatalf("blocked = %#v, found = %t, err = %v", blocked, found, err)
			}
		})
	}
}

// A rule line in the conversation, of the same width as the permission
// dialog's border and followed by a ❯ line, opens a frame that the dialog's
// border closes, so the screen reads as a composer.
func TestBlockedFindsAClaudePromptBelowAComposerShapedFrame(t *testing.T) {
	collar, err := EmbeddedCollar("claude")
	if err != nil {
		t.Fatal(err)
	}
	dialog := fixtureLines(t, "claude-code-2.1.287-permission-bash.txt")[5:]
	lines := append([]string{strings.Repeat("─", 160), "❯ run the build", ""}, dialog...)
	if _, err := ReadComposer(collar.Primitives.Composer, linesScreen(lines)); err != nil {
		t.Fatalf("frame above the dialog does not read as a composer: %v", err)
	}
	cursors := map[string]substrate.Cursor{
		"no cursor":                       {},
		"hidden cursor in the frame":      {Row: 1, Column: 2},
		"cursor in a dialog's text field": {Row: 10, Column: 2, Visible: true},
	}
	for name, cursor := range cursors {
		t.Run(name, func(t *testing.T) {
			screen := linesScreen(lines)
			screen.Cursor = cursor
			blocked, found, err := InputBlocked(collar, screen)
			if err != nil || !found {
				t.Fatalf("blocked = %#v, found = %t, err = %v", blocked, found, err)
			}
		})
	}
}

func TestBlockedEvidenceNamesThePromptNotTheScrollback(t *testing.T) {
	collar, err := EmbeddedCollar("claude")
	if err != nil {
		t.Fatal(err)
	}
	lines := append(fixtureLines(t, "claude-code-2.1.287-pattern-in-reply.txt"), fixtureLines(t, "claude-code-2.1.287-permission-bash.txt")...)
	blocked, found, err := InputBlocked(collar, linesScreen(lines))
	if err != nil || !found {
		t.Fatalf("blocked = %#v, found = %t, err = %v", blocked, found, err)
	}
	if blocked.Evidence != "native input choice: Do you want to proceed?; ❯ 1. Yes" {
		t.Fatalf("evidence = %q", blocked.Evidence)
	}
}

func TestTrustTextInTheConversationDoesNotBlockInput(t *testing.T) {
	codexComposer := fixtureLines(t, "codex-0.151.0-composer.txt")
	directoryTrust := fixtureLines(t, "codex-0.151.0-directory-trust.txt")
	quoted := func(intro string, lines []string) []string {
		return append(quotedLines(intro, lines), codexComposer...)
	}
	tests := []struct {
		name   string
		collar string
		lines  []string
	}{
		{name: "claude history above an idle composer", collar: "claude", lines: append([]string{"❯ Yes, read it", "Do you trust the contents of this directory?"},
			fixtureLines(t, "claude-code-2.1.287-pattern-in-reply.txt")...)},
		{name: "codex reply quoting directory trust", collar: "codex", lines: quoted("• The trust screen reads:", directoryTrust)},
		{name: "codex reply quoting hook trust", collar: "codex", lines: quoted("• The hook screen reads:", fixtureLines(t, "codex-0.151.0-hook-trust.txt"))},
		{name: "codex history above an idle composer", collar: "codex", lines: append([]string{"› Yes, continue with the plan", "", "• Codex asks:", directoryTrust[2], ""}, codexComposer...)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			collar, err := EmbeddedCollar(test.collar)
			if err != nil {
				t.Fatal(err)
			}
			screen := linesScreen(test.lines)
			if test.collar == "codex" {
				screen = codexComposerScreen(test.lines)
			}
			blocked, found, err := InputBlocked(collar, screen)
			if err != nil || found {
				t.Fatalf("blocked = %#v, found = %t, err = %v", blocked, found, err)
			}
		})
	}
}

func TestTrustPromptsBlockInput(t *testing.T) {
	tests := []struct {
		collar, file string
		cursor       substrate.Cursor
	}{
		{collar: "claude", file: "claude-code-2.1.278-directory-trust.txt"},
		{collar: "codex", file: "codex-0.151.0-directory-trust.txt"},
		{collar: "codex", file: "codex-0.151.0-hook-trust.txt"},
		// Codex draws the selected row of its hooks list with the same › as
		// its composer and parks the hidden cursor at or below that row.
		{collar: "codex", file: "codex-0.160.0-hooks-list.txt", cursor: substrate.Cursor{Row: 23, Column: 80}},
		{collar: "codex", file: "codex-0.160.0-hooks-list-first-row.txt", cursor: substrate.Cursor{Row: 14, Column: 80}},
	}
	for _, test := range tests {
		t.Run(test.file, func(t *testing.T) {
			collar, err := EmbeddedCollar(test.collar)
			if err != nil {
				t.Fatal(err)
			}
			screen := fixtureScreen(t, test.file)
			screen.Cursor = test.cursor
			blocked, found, err := InputBlocked(collar, screen)
			if err != nil || !found {
				t.Fatalf("blocked = %#v, found = %t, err = %v", blocked, found, err)
			}
		})
	}
}

func TestBlockedPrimitiveRequiresPromptAndChoice(t *testing.T) {
	collar := Collar{Primitives: Primitives{
		Composer: Invocation{Name: "codex-composer"},
		Blocked:  Invocation{Name: "screen-blocked", Params: map[string]string{"prompt": "approve", "choice": "yes"}},
	}}
	_, found, err := InputBlocked(collar, testScreen(testCells("approve this command", false)))
	if err != nil {
		t.Fatal(err)
	}
	if found {
		t.Fatal("prompt without a choice was detected as blocked")
	}
}

// quotedLines renders lines the way a reply quotes a screen: an introduction,
// a blank line, and the quoted lines indented as a code block.
func quotedLines(intro string, lines []string) []string {
	reply := []string{intro, ""}
	for _, line := range lines {
		reply = append(reply, "    "+line)
	}
	return reply
}

// codexComposerScreen shows lines with the visible cursor on the last › row,
// where Codex draws it while its composer holds input.
func codexComposerScreen(lines []string) substrate.Screen {
	screen := linesScreen(lines)
	for index := len(lines) - 1; index >= 0; index-- {
		if strings.HasPrefix(lines[index], "›") {
			screen.Cursor = substrate.Cursor{Row: index, Column: 2, Visible: true}
			break
		}
	}
	return screen
}

func fixtureLines(t *testing.T, name string) []string {
	t.Helper()
	return screenLines(fixtureScreen(t, name), true)
}

func linesScreen(lines []string) substrate.Screen {
	rows := make([][]substrate.Cell, len(lines))
	for index, line := range lines {
		rows[index] = testCells(strings.TrimRight(line, " "), false)
	}
	return testScreen(rows...)
}
