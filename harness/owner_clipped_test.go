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

const wrappedMainOwner = "i112 probe main"

// wrappedScreen is a named main session's composer in a 90x14 pane after a
// literal message wrapped it onto three rows and pushed the mode line off the
// pane, with its rule relabelled to label.
func wrappedScreen(t *testing.T, label string) substrate.Screen {
	t.Helper()
	screen := fixtureScreen(t, "claude-named-wrapped-short-pane.txt")
	for index, row := range screen.Rows {
		line := screenLines(substrate.Screen{Rows: [][]substrate.Cell{row}}, false)[0]
		if strings.Contains(line, wrappedMainOwner) {
			screen.Rows[index] = testCells(strings.Repeat("─", 87-len([]rune(label)))+" "+label+" ─", false)
		}
	}
	return screen
}

func TestClaudeComposerOwnerClippedInShortPane(t *testing.T) {
	composer, err := ReadComposer(Invocation{Name: "claude-composer"}, fixtureScreen(t, "claude-named-wrapped-short-pane.txt"))
	if !errors.Is(err, ErrComposerOwnerClipped) {
		t.Fatalf("error = %v, want %v", err, ErrComposerOwnerClipped)
	}
	if composer.Owner != wrappedMainOwner || !strings.HasSuffix(composer.Text, "with: ok. [/gang:self-declared:operator#e85f6ef9d930c633]") {
		t.Fatalf("composer = %+v", composer)
	}

	// A longer message pushes the whole footer off the pane.
	screen := fixtureScreen(t, "claude-named-wrapped-short-pane.txt")
	screen.Rows = screen.Rows[:len(screen.Rows)-1]
	if composer, err := ReadComposer(Invocation{Name: "claude-composer"}, screen); !errors.Is(err, ErrComposerOwnerClipped) || composer.Owner != wrappedMainOwner {
		t.Fatalf("closing rule on the last row: composer = %+v, error = %v, want %v", composer, err, ErrComposerOwnerClipped)
	}

	// Below the pane's last row the footer may hold a session list, so a
	// footer that ends above it still rules the composer foreign.
	screen = fixtureScreen(t, "claude-named-wrapped-short-pane.txt")
	screen.Rows = append(screen.Rows, testCells("", false))
	if _, err := ReadComposer(Invocation{Name: "claude-composer"}, screen); !errors.Is(err, ErrForeignComposer) {
		t.Fatalf("footer above the last row: error = %v, want %v", err, ErrForeignComposer)
	}
}

func TestOwnedComposerSettleAcceptsClippedFooterOnlyForItsOwner(t *testing.T) {
	collar, err := EmbeddedCollar("claude")
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name, owner, label string
		err                error
	}{
		{name: "same main", owner: wrappedMainOwner, label: wrappedMainOwner},
		{name: "no recorded owner", owner: "", label: wrappedMainOwner, err: ErrComposerOwnerClipped},
		{name: "unlabelled rule without a recorded owner", owner: "", label: "", err: ErrComposerOwnerClipped},
		{name: "child view", owner: wrappedMainOwner, label: "Read all .go files in harness/ and cmd/gang/ with one-sentence summaries", err: ErrComposerOwnerClipped},
	} {
		t.Run(test.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()
				screen := wrappedScreen(t, test.label)
				err := AwaitOwnedComposerSettle(ctx, func(context.Context, substrate.PaneID) (substrate.Screen, error) {
					return screen, nil
				}, "%1", collar, test.owner, 400*time.Millisecond)
				if test.err == nil && err != nil || test.err != nil && !errors.Is(err, test.err) {
					t.Fatalf("error = %v, want %v", err, test.err)
				}
			})
		})
	}
}

// A repaint that stalls partway can show a torn screen that reads as a
// clipped or foreign composer. The wait refuses only a verdict that holds for
// its settle window, and reads a shorter one as an unreadable frame.
func TestOwnedComposerSettleHoldsARefusalForItsWindow(t *testing.T) {
	collar, err := EmbeddedCollar("claude")
	if err != nil {
		t.Fatal(err)
	}
	const settle = 400 * time.Millisecond
	child := wrappedScreen(t, "Read the harness sources")
	main := wrappedScreen(t, wrappedMainOwner)
	unreadable := textScreen("Transcript view", "  earlier reply")
	for _, test := range []struct {
		name string
		// screen returns the screen at a time since the wait began.
		screen  func(time.Duration) substrate.Screen
		elapsed time.Duration
		err     error
	}{
		{name: "refused for less than the window", screen: func(at time.Duration) substrate.Screen {
			if at < 300*time.Millisecond {
				return child
			}
			return main
		}, elapsed: 300*time.Millisecond + settle},
		{name: "refused for the window", screen: func(time.Duration) substrate.Screen { return child }, elapsed: settle, err: ErrComposerOwnerClipped},
		// An unreadable frame restarts the window.
		{name: "refused around an unreadable gap", screen: func(at time.Duration) substrate.Screen {
			if at >= 100*time.Millisecond && at < 200*time.Millisecond {
				return unreadable
			}
			return child
		}, elapsed: 200*time.Millisecond + settle, err: ErrComposerOwnerClipped},
		// Isolated torn frames never add up to a refusal.
		{name: "refusing frames apart", screen: func(at time.Duration) substrate.Screen {
			switch {
			case at < 50*time.Millisecond, at >= 3*time.Second && at < 3050*time.Millisecond:
				return child
			case at < 3*time.Second:
				return unreadable
			}
			return main
		}, elapsed: 3050*time.Millisecond + settle},
		// A deadline that comes during unreadable frames reports the last
		// frame's verdict, not a refusal that never held.
		{name: "refused, then unreadable to the deadline", screen: func(at time.Duration) substrate.Screen {
			if at >= 4800*time.Millisecond && at < 4900*time.Millisecond {
				return child
			}
			return unreadable
		}, elapsed: 5 * time.Second, err: ErrNoComposer},
	} {
		t.Run(test.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				began := time.Now()
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				err := AwaitOwnedComposerSettle(ctx, func(context.Context, substrate.PaneID) (substrate.Screen, error) {
					return test.screen(time.Since(began)), nil
				}, "%1", collar, wrappedMainOwner, settle)
				if test.err == nil && err != nil || test.err != nil && !errors.Is(err, test.err) {
					t.Fatalf("error = %v, want %v", err, test.err)
				}
				if test.err != ErrComposerOwnerClipped && errors.Is(err, ErrComposerOwnerClipped) {
					t.Fatalf("error = %v, want no refusal", err)
				}
				if elapsed := time.Since(began); elapsed != test.elapsed {
					t.Fatalf("returned after %v, want %v", elapsed, test.elapsed)
				}
			})
		})
	}
}
