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
			blocked, found, err := InputBlocked(collar, linesScreen(test.lines))
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
	blocked, found, err := InputBlocked(collar, linesScreen(lines))
	if err != nil || !found {
		t.Fatalf("blocked = %#v, found = %t, err = %v", blocked, found, err)
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
