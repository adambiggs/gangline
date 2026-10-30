package harness

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"
	"time"

	"github.com/adambiggs/gangline/substrate"
)

func TestOwnedComposerSettleDoesNotWaitForPasteHintExpiry(t *testing.T) {
	for _, test := range []struct {
		name, owner, fixture string
		want                 error
	}{
		{"same main", "Test agent assignment", "claude-named-paste-hint.txt", nil},
		{"unknown owner", "", "claude-named-paste-hint.txt", context.DeadlineExceeded},
		{"changed owner", "Another session", "claude-named-paste-hint.txt", context.DeadlineExceeded},
		{"child view", "Test agent assignment", "claude-subagent-paste-hint.txt", context.DeadlineExceeded},
		{"selected child", "Read harness files", "claude-subagent-listed-paste-hint.txt", ErrForeignComposer},
	} {
		t.Run(test.name, func(t *testing.T) {
			screen := fixtureScreen(t, test.fixture)
			synctest.Test(t, func(t *testing.T) {
				collar, err := EmbeddedCollar("claude")
				if err != nil {
					t.Fatal(err)
				}
				start := time.Now()
				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()
				err = AwaitOwnedComposerSettle(ctx, func(context.Context, substrate.PaneID) (substrate.Screen, error) {
					return screen, nil
				}, "%1", collar, test.owner, 400*time.Millisecond)
				if !errors.Is(err, test.want) {
					t.Fatalf("error=%v; want %v", err, test.want)
				}
				if test.want == nil && time.Since(start) != 400*time.Millisecond {
					t.Fatalf("settle took %s; want 400ms with 600ms before fake deadline", time.Since(start))
				}
			})
		})
	}
}
