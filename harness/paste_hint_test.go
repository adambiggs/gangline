package harness

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"
	"time"

	"github.com/adambiggs/gangline/substrate"
)

func TestClaudePasteHintHidesComposerOwner(t *testing.T) {
	for _, file := range []string{"claude-named-paste-hint.txt", "claude-subagent-paste-hint.txt"} {
		t.Run(file, func(t *testing.T) {
			if _, err := ReadComposer(Invocation{Name: "claude-composer"}, fixtureScreen(t, file)); !errors.Is(err, ErrComposerOwnerHidden) {
				t.Fatalf("error = %v, want %v", err, ErrComposerOwnerHidden)
			}
		})
	}
}

// The paste hint replaces the footer that names the composer's session, so
// settling waits for the footer and then applies the ownership check to it.
// A session list that selects a child names the owner the hint hides.
func TestClaudePasteHintWithListedChildIsForeign(t *testing.T) {
	if _, err := ReadComposer(Invocation{Name: "claude-composer"}, fixtureScreen(t, "claude-subagent-listed-paste-hint.txt")); !errors.Is(err, ErrForeignComposer) {
		t.Fatalf("error = %v, want %v", err, ErrForeignComposer)
	}
}

func TestComposerSettleChecksOwnerAfterPasteHint(t *testing.T) {
	for _, test := range []struct {
		name  string
		after string
		want  error
	}{
		{"main session", "claude-named-paste-hint-cleared.txt", nil},
		{"child session", "claude-selected-subagent.txt", ErrForeignComposer},
	} {
		t.Run(test.name, func(t *testing.T) {
			hint, after := fixtureScreen(t, "claude-named-paste-hint.txt"), fixtureScreen(t, test.after)
			synctest.Test(t, func(t *testing.T) {
				collar, _ := EmbeddedCollar("claude")
				start := time.Now()
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				capture := func(context.Context, substrate.PaneID) (substrate.Screen, error) {
					if time.Since(start) < time.Second {
						return hint, nil
					}
					return after, nil
				}
				err := AwaitComposerSettle(ctx, capture, "%1", collar, 400*time.Millisecond)
				if !errors.Is(err, test.want) {
					t.Fatalf("error = %v, want %v", err, test.want)
				}
				// Fake time: the hint frames until 1s, then the footer is read.
				// The main session settles after its 400ms window; the child is
				// refused on its first frame.
				want := time.Second
				if test.want == nil {
					want += 400 * time.Millisecond
				}
				if elapsed := time.Since(start); elapsed != want {
					t.Fatalf("returned after %s, want %s", elapsed, want)
				}
			})
		})
	}
}
