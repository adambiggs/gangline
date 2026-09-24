package harness

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"
	"time"

	"github.com/adambiggs/gangline/substrate"
)

func TestComposerSettleSurvivesMissingFrames(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		collar, _ := EmbeddedCollar("codex")
		start := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		capture := func(context.Context, substrate.PaneID) (substrate.Screen, error) {
			elapsed := time.Since(start)
			// Missing initially and again partway through settling. The second
			// gap must restart the entire stability window.
			if elapsed < 25*time.Millisecond || elapsed >= 225*time.Millisecond && elapsed < 250*time.Millisecond {
				return testScreen(testCells("repainting", false)), nil
			}
			return testScreen(testCells("› message", false)), nil
		}
		if err := AwaitComposerSettle(ctx, capture, "%1", collar, 400*time.Millisecond); err != nil {
			t.Fatal(err)
		}
		// Fake time: 250ms to the final repaint + 400ms stable, leaving
		// 350ms before the deadline. No wall-clock wait or scheduling margin.
		if elapsed := time.Since(start); elapsed != 650*time.Millisecond {
			t.Fatalf("settled after %s, want 650ms", elapsed)
		}
	})
}

func TestComposerSettleMissingUntilDeadline(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		collar, _ := EmbeddedCollar("codex")
		start := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		err := AwaitComposerSettle(ctx, func(context.Context, substrate.PaneID) (substrate.Screen, error) {
			return testScreen(testCells("unknown surface", false)), nil
		}, "%1", collar, 400*time.Millisecond)
		if !errors.Is(err, context.DeadlineExceeded) || !errors.Is(err, ErrNoComposer) {
			t.Fatalf("missing timeout or parse diagnostic: %v", err)
		}
		// The fake deadline is exact: zero overrun and no real timeout.
		if elapsed := time.Since(start); elapsed != time.Second {
			t.Fatalf("deadline elapsed after %s", elapsed)
		}
	})
}

func TestComposerSettleStopsOnUnsafeSurfaceOrCaptureFailure(t *testing.T) {
	collar, _ := EmbeddedCollar("codex")
	captureErr := errors.New("pane disappeared")
	for _, test := range []struct {
		name    string
		screen  substrate.Screen
		capture error
		want    error
	}{
		{"choice", testScreen(testCells("› 1. Yes, proceed", false)), nil, ErrComposerOccupied},
		{"capture", substrate.Screen{}, captureErr, captureErr},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			err := AwaitComposerSettle(ctx, func(context.Context, substrate.PaneID) (substrate.Screen, error) {
				return test.screen, test.capture
			}, "%1", collar, 400*time.Millisecond)
			if !errors.Is(err, test.want) {
				t.Fatalf("got %v, want %v", err, test.want)
			}
		})
	}
}
