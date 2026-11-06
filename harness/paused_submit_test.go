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
// which the composer cannot be read in full. Only evidence that the submit
// key would reach something other than that composer withholds it.
func TestPausedSubmitRefusal(t *testing.T) {
	collar := Collar{Primitives: Primitives{Composer: Invocation{Name: "claude-composer"}}}
	rule := strings.Repeat("─", 40)
	for _, test := range []struct {
		name   string
		screen substrate.Screen
		owner  string
		// read is ReadComposer's error on the screen, which picks the case.
		read error
		want error
	}{
		{name: "readable paste", screen: textScreen(rule, "❯ [gang:lead#1] hello", rule, "  ? for shortcuts")},
		{name: "closing rule below the pane", read: ErrComposerClipped, screen: textScreen("older output", rule, "❯ [gang:lead#1] a long message that", "  wraps past the bottom of the pane")},
		{name: "tall frame without a cursor", read: ErrNoComposer, screen: textScreen(rule, "❯ line 1", "  line 2", "  line 3", "  line 4", "  line 5", "  line 6", "  line 7", rule)},
		{name: "footer clipped, same owner", screen: wrappedScreen(t, wrappedMainOwner), owner: wrappedMainOwner, read: ErrComposerOwnerClipped},
		{name: "footer clipped, owner unknown", screen: wrappedScreen(t, wrappedMainOwner), read: ErrComposerOwnerClipped},
		{name: "footer clipped, child view", screen: wrappedScreen(t, "Read the harness sources"), owner: wrappedMainOwner, read: ErrComposerOwnerClipped, want: ErrForeignComposer},
		{name: "overlay owns input", screen: fixtureScreen(t, "claude-auto-mode-environment.txt"), read: ErrComposerOccupied, want: ErrComposerOccupied},
		{name: "background session composer", screen: fixtureScreen(t, "claude-background-sessions.txt"), read: ErrBackgroundComposer, want: ErrBackgroundComposer},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := ReadComposer(collar.Primitives.Composer, test.screen); !errors.Is(err, test.read) {
				t.Fatalf("read error = %v, want %v", err, test.read)
			}
			err := PausedSubmitRefusal(collar, test.screen, test.owner)
			if test.want == nil && err != nil || test.want != nil && !errors.Is(err, test.want) {
				t.Fatalf("error = %v, want %v", err, test.want)
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
