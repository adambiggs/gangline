package harness

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/adambiggs/gangline/substrate"
)

func TestNativeWaitsReturnCaptureFailure(t *testing.T) {
	want := errors.New("tmux capture lost its pane")
	capture := func(context.Context, substrate.PaneID) (substrate.Screen, error) { return substrate.Screen{}, want }
	collar, err := EmbeddedCollar("codex")
	if err != nil {
		t.Fatal(err)
	}
	for name, wait := range map[string]func(context.Context) error{
		"composer text": func(ctx context.Context) error {
			return AwaitComposerText(ctx, capture, "%1", collar, "hello", time.Second)
		},

		"startup": func(ctx context.Context) error { _, _, err := AwaitStartup(ctx, capture, "%1", collar); return err },
	} {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			if err := wait(ctx); !errors.Is(err, want) {
				t.Fatalf("capture failure lost: %v", err)
			}
		})
	}
}

func TestComposerWaitRetainsParseFailure(t *testing.T) {
	collar, _ := EmbeddedCollar("codex")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := AwaitComposerText(ctx, func(context.Context, substrate.PaneID) (substrate.Screen, error) {
		return testScreen(testCells("unknown native surface", false)), nil
	}, "%1", collar, "hello", time.Second)
	if !errors.Is(err, ErrNoComposer) {
		t.Fatalf("parse diagnostic lost: %v", err)
	}
}
