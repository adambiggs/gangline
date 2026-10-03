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
