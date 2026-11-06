package harness

import (
	"context"
	"errors"
	"strings"
	"testing"
	"testing/synctest"
	"time"

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
	// Only the Claude Code composer reader recognises a clipped paste.
	other := collar
	other.Primitives.Composer.Name = "codex-composer"
	if err := validateCollar(other); err == nil || !strings.Contains(err.Error(), "before_submit") {
		t.Fatalf("pause with another composer: %v", err)
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
		{name: "readable paste", screen: textScreen(rule, start, rule, "  ? for shortcuts"), shown: true},
		{name: "readable paste placeholder", screen: textScreen(rule, "❯ [Pasted text #1 +12 lines]", rule, "  ? for shortcuts"), shown: true},
		{name: "readable image placeholder", screen: textScreen(rule, "❯ [Image #1] look at this", rule, "  ? for shortcuts")},
		{name: "readable other text", screen: textScreen(rule, "❯ a draft typed after the paste was cleared", rule, "  ? for shortcuts")},
		{name: "empty composer", screen: textScreen(rule, "❯ ", rule, "  ? for shortcuts")},
		{name: "closing rule below the pane", read: ErrComposerClipped, screen: textScreen("older output", rule, start, "  message that wraps past the bottom"), shown: true},
		{name: "closing rule below the pane, named owner", owner: "probe main", read: ErrNoComposer, screen: textScreen("older output", named("probe main"), start, "  message that wraps past the bottom"), shown: true},
		{name: "closing rule below the pane, other text", read: ErrComposerClipped, screen: textScreen("older output", rule, "❯ [gang:lead#fedcba9876543210] a long", "  message that wraps past the bottom")},
		{name: "closing rule below the pane, child rule", owner: "probe main", read: ErrNoComposer, screen: textScreen("older output", named("Read the sources"), start, "  message that wraps past the bottom")},
		{name: "closing rule below the pane, named after an unnamed read", read: ErrNoComposer, screen: textScreen("older output", named("Read the sources"), start, "  message that wraps past the bottom")},
		{name: "tall frame without a cursor", read: ErrNoComposer, screen: textScreen(rule, start, "  message", "  that", "  wraps", "  past", "  the", "  bottom", rule), shown: true},
		{name: "rule narrower than another row", read: ErrComposerClipped, screen: textScreen("a transcript row wider than the short rule above the prompt", rule, start, "  message"), shown: true},
		{name: "closing rule below the pane, earlier long paste", read: ErrComposerClipped, screen: textScreen("older output", rule, "❯ [Pasted text #1 +40 lines]", "  message that wraps past the bottom")},
		// The same row may be this paste; it is not told apart, so it stays
		// unverified rather than risk the key on an earlier paste.
		{name: "closing rule below the pane, placeholder only", read: ErrComposerClipped, screen: textScreen("older output", rule, "❯ [Pasted text #2 +40 lines]")},
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
			wire := wire
			if strings.HasPrefix(test.name, "footer clipped") {
				// The fixture holds a message of its own.
				composer, _ := ReadComposer(collar.Primitives.Composer, test.screen)
				wire = composer.Text
			}
			shown, err := PasteShown(collar, test.screen, wire, test.owner)
			if shown != test.shown || test.want == nil && err != nil || test.want != nil && !errors.Is(err, test.want) {
				t.Fatalf("shown = %v, error = %v; want %v, %v", shown, err, test.shown, test.want)
			}
		})
	}
}

// The wait before the submit key returns once the pause has passed and the
// paste shows. A paste that shows and then leaves the composer, or a
// composer that holds other text at the deadline, gets no key.
func TestAwaitPasteShown(t *testing.T) {
	collar := Collar{Primitives: Primitives{Composer: Invocation{Name: "claude-composer"}}}
	rule := strings.Repeat("─", 40)
	const wire = "[gang:lead#0123456789abcdef] hello [/gang:lead#0123456789abcdef]"
	pasted := textScreen(rule, "❯ [gang:lead#0123456789abcdef] hello", rule, "  ? for shortcuts")
	empty := textScreen(rule, "❯ ", rule, "  ? for shortcuts")
	draft := textScreen(rule, "❯ a draft", rule, "  ? for shortcuts")
	const settle = 400 * time.Millisecond
	for _, test := range []struct {
		name string
		// screen returns the screen at a time since the wait began.
		screen  func(time.Duration) substrate.Screen
		elapsed time.Duration
		want    error
	}{
		{name: "shown", screen: func(time.Duration) substrate.Screen { return pasted }, elapsed: settle},
		{name: "shown late", screen: func(at time.Duration) substrate.Screen {
			if at < time.Second {
				return empty
			}
			return pasted
		}, elapsed: time.Second},
		{name: "shown then emptied", screen: func(at time.Duration) substrate.Screen {
			if at < 100*time.Millisecond {
				return pasted
			}
			return empty
		}, elapsed: 500 * time.Millisecond, want: ErrComposerEmptied},
		{name: "replaced", screen: func(time.Duration) substrate.Screen { return draft }, elapsed: settle, want: ErrComposerReplaced},
		{name: "replaced around an unreadable gap", screen: func(at time.Duration) substrate.Screen {
			if at >= 100*time.Millisecond && at < 450*time.Millisecond {
				return textScreen("Transcript view", "  earlier reply")
			}
			return draft
		}, elapsed: 850 * time.Millisecond, want: ErrComposerReplaced},
		{name: "replaced, then unreadable to the deadline", screen: func(at time.Duration) substrate.Screen {
			if at < 100*time.Millisecond {
				return draft
			}
			return textScreen("Transcript view", "  earlier reply")
		}, elapsed: 5 * time.Second, want: ErrComposerReplaced},
		{name: "never shown", screen: func(time.Duration) substrate.Screen { return empty }, elapsed: 5 * time.Second, want: context.DeadlineExceeded},
	} {
		t.Run(test.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				began := time.Now()
				ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
				defer cancel()
				capture := func(context.Context, substrate.PaneID) (substrate.Screen, error) {
					return test.screen(time.Since(began)), nil
				}
				err := AwaitPasteShown(ctx, capture, "%1", collar, wire, "", settle)
				if test.want == nil && err != nil || test.want != nil && !errors.Is(err, test.want) {
					t.Fatalf("error = %v, want %v", err, test.want)
				}
				if elapsed := time.Since(began); elapsed != test.elapsed {
					t.Fatalf("returned after %v, want %v", elapsed, test.elapsed)
				}
			})
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
