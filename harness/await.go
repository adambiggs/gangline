package harness

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/adambiggs/gangline/substrate"
)

type captureScreen func(context.Context, substrate.PaneID) (substrate.Screen, error)

// AwaitComposerText waits until the native composer shows stable expected text.
func AwaitComposerText(ctx context.Context, capture captureScreen, pane substrate.PaneID, collar Collar, want string, settle time.Duration) error {
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	var stableSince time.Time
	for {
		screen, err := capture(ctx, pane)
		if err == nil {
			composer, readErr := ReadComposer(collar.Primitives.Composer, screen)
			if readErr == nil && composer.Text == want {
				if stableSince.IsZero() {
					stableSince = time.Now()
				}
				if time.Since(stableSince) >= settle {
					return nil
				}
			} else {
				stableSince = time.Time{}
			}
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("native composer did not show submitted text: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}

// AwaitStartup waits for an observable ready or trust-required startup state.
func AwaitStartup(ctx context.Context, capture captureScreen, pane substrate.PaneID, collar Collar) (Startup, substrate.Screen, error) {
	const readySettle = 400 * time.Millisecond
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	var lastErr error
	var readySince time.Time
	for {
		screen, err := capture(ctx, pane)
		if err == nil {
			startup, inspectErr := InspectStartup(collar, screen)
			if inspectErr == nil && startup.State == StartupTrustRequired {
				return startup, screen, nil
			}
			if inspectErr == nil && startup.State == StartupReady {
				if readySince.IsZero() {
					readySince = time.Now()
				}
				if time.Since(readySince) >= readySettle {
					return startup, screen, nil
				}
			} else {
				readySince = time.Time{}
			}
			if inspectErr != nil {
				lastErr = inspectErr
			} else {
				lastErr = errors.New(startup.Prompt)
			}
		} else {
			lastErr = err
		}
		select {
		case <-ctx.Done():
			return Startup{}, substrate.Screen{}, ctx.Err()
		case <-deadline.C:
			return Startup{}, substrate.Screen{}, fmt.Errorf("native startup was not observable within 5s: %w", lastErr)
		case <-ticker.C:
		}
	}
}

// AwaitScreenSettle waits until a changed native screen remains stable.
func AwaitScreenSettle(ctx context.Context, capture captureScreen, pane substrate.PaneID, before substrate.Screen, settle time.Duration) error {
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	var last substrate.Screen
	var stableSince time.Time
	for {
		screen, err := capture(ctx, pane)
		if err == nil && !reflect.DeepEqual(screen, before) {
			if stableSince.IsZero() || !reflect.DeepEqual(screen, last) {
				stableSince = time.Now()
				last = screen
			}
			if time.Since(stableSince) >= settle {
				return nil
			}
		} else {
			stableSince = time.Time{}
			last = substrate.Screen{}
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("native composer did not settle before submission: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}
