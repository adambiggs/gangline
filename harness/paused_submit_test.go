package harness

import (
	"errors"
	"strings"
	"testing"

	"github.com/adambiggs/gangline/substrate"
)

func TestSubmitPausesMode(t *testing.T) {
	for _, test := range []struct {
		mode   string
		pauses bool
		err    bool
	}{
		{mode: "", pauses: false},
		{mode: "pause", pauses: true},
		{mode: "wait", err: true},
	} {
		invocation := Invocation{Name: "enter-submit", Params: map[string]string{"before_submit": test.mode}}
		pauses, err := SubmitPauses(invocation)
		if pauses != test.pauses || (err != nil) != test.err {
			t.Fatalf("mode %q: pauses=%v err=%v", test.mode, pauses, err)
		}
	}
	if _, err := SubmitPauses(Invocation{Name: "other"}); err == nil {
		t.Fatal("unknown submit primitive accepted")
	}
}

func TestClaudeCollarPausesBeforeSubmit(t *testing.T) {
	collar, err := EmbeddedCollar("claude")
	if err != nil {
		t.Fatal(err)
	}
	if pauses, err := SubmitPauses(collar.Primitives.Submit); err != nil || !pauses {
		t.Fatalf("Claude collar pauses=%v err=%v", pauses, err)
	}
}

// A paste into the empty composer of a short pane can leave a screen on
// which the composer cannot be read in full. The paste is shown once its start
// appears under the owner's rule; a screen without it is not shown yet, and
// only evidence that the key would reach something else refuses.
func TestPasteShown(t *testing.T) {
	collar := Collar{Primitives: Primitives{Composer: Invocation{Name: "claude-composer"}}}
	rule := strings.Repeat("─", 40)
	named := func(label string) string { return strings.Repeat("─", 37-len([]rune(label))) + " " + label + " ─" }
	const wire = "[gang:lead#0123456789abcdef] a long message that wraps past the bottom of the pane [/gang:lead#0123456789abcdef]"
	start := "❯ [gang:lead#0123456789abcdef] a long"
	for _, test := range []struct {
		name   string
		screen substrate.Screen
		owner  string
		// read is ReadComposer's error on the screen, which picks the case.
		read  error
		shown bool
		want  error
	}{
		{name: "readable paste", screen: textScreen(rule, "❯ [gang:lead#0123456789abcdef] hi", rule, "  ? for shortcuts"), shown: true},
		{name: "empty composer", screen: textScreen(rule, "❯ ", rule, "  ? for shortcuts")},
		{name: "closing rule below the pane", read: ErrComposerClipped, screen: textScreen("older output", rule, start, "  message that wraps past the bottom"), shown: true},
		{name: "closing rule below the pane, named owner", owner: "probe main", read: ErrNoComposer, screen: textScreen("older output", named("probe main"), start, "  message that wraps past the bottom"), shown: true},
		{name: "closing rule below the pane, other text", read: ErrComposerClipped, screen: textScreen("older output", rule, "❯ [gang:lead#fedcba9876543210] a long", "  message that wraps past the bottom")},
		{name: "closing rule below the pane, child rule", owner: "probe main", read: ErrNoComposer, screen: textScreen("older output", named("Read the sources"), start, "  message that wraps past the bottom")},
		{name: "closing rule below the pane, named after an unnamed read", read: ErrNoComposer, screen: textScreen("older output", named("Read the sources"), start, "  message that wraps past the bottom")},
		{name: "tall frame without a cursor", read: ErrNoComposer, screen: textScreen(rule, start, "  message", "  that", "  wraps", "  past", "  the", "  bottom", rule), shown: true},
		{name: "full-screen view", read: ErrNoComposer, screen: fixtureScreen(t, "claude-code-2.1.287-transcript-view.txt")},
		{name: "footer clipped, same owner", screen: wrappedScreen(t, wrappedMainOwner), owner: wrappedMainOwner, read: ErrComposerOwnerClipped, shown: true},
		{name: "footer clipped after an unnamed read", screen: wrappedScreen(t, wrappedMainOwner), read: ErrComposerOwnerClipped, want: ErrForeignComposer},
		{name: "footer clipped, child view", screen: wrappedScreen(t, "Read the harness sources"), owner: wrappedMainOwner, read: ErrComposerOwnerClipped, want: ErrForeignComposer},
		{name: "overlay owns input", screen: fixtureScreen(t, "claude-auto-mode-environment.txt"), read: ErrComposerOccupied, want: ErrComposerOccupied},
		{name: "background session composer", screen: fixtureScreen(t, "claude-background-sessions.txt"), read: ErrBackgroundComposer, want: ErrBackgroundComposer},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := ReadComposer(collar.Primitives.Composer, test.screen); !errors.Is(err, test.read) {
				t.Fatalf("read error = %v, want %v", err, test.read)
			}
			shown, err := PasteShown(collar, test.screen, wire, test.owner)
			if shown != test.shown || test.want == nil && err != nil || test.want != nil && !errors.Is(err, test.want) {
				t.Fatalf("shown = %v, error = %v; want %v, %v", shown, err, test.shown, test.want)
			}
		})
	}
}

func textScreen(lines ...string) substrate.Screen {
	rows := make([][]substrate.Cell, len(lines))
	for index, line := range lines {
		rows[index] = testCells(line, false)
	}
	return testScreen(rows...)
}
