package harness

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/adambiggs/gangline/substrate"
)

type captureScreen func(context.Context, substrate.PaneID) (substrate.Screen, error)

// Idle requires a recognized empty composer and no declared native busy marker.
func Idle(collar Collar, screen substrate.Screen) (bool, error) {
	composer, err := ReadComposer(collar.Primitives.Composer, screen)
	if err != nil {
		return false, err
	}
	pattern := collar.Primitives.Wedge.Params["busy"]
	if pattern == "" {
		return false, fmt.Errorf("collar has no native busy expression")
	}
	busy, err := regexp.Compile(pattern)
	if err != nil {
		return false, err
	}
	return composer.Text == "" && !busy.MatchString(strings.Join(screenLines(screen, true), "\n")), nil
}

func AwaitIdle(ctx context.Context, capture captureScreen, pane substrate.PaneID, collar Collar) error {
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		screen, err := capture(ctx, pane)
		if err != nil {
			return err
		}
		idle, err := Idle(collar, screen)
		if err != nil {
			return err
		}
		if idle {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("native turn did not become idle after interruption: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}

// AwaitComposerText waits until the native composer shows stable expected text.
func AwaitComposerText(ctx context.Context, capture captureScreen, pane substrate.PaneID, collar Collar, want string, settle time.Duration) error {
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	var stableSince time.Time
	var lastErr error
	for {
		screen, err := capture(ctx, pane)
		if err != nil {
			return fmt.Errorf("observe native composer: %w", err)
		}
		composer, readErr := ReadComposer(collar.Primitives.Composer, screen)
		lastErr = readErr
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
		select {
		case <-ctx.Done():
			return fmt.Errorf("native composer did not show submitted text: %w", errors.Join(ctx.Err(), lastErr))
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
			return Startup{}, substrate.Screen{}, fmt.Errorf("observe native startup: %w", err)
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
		if err != nil {
			return fmt.Errorf("observe native screen settling: %w", err)
		}
		if !reflect.DeepEqual(screen, before) {
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

// AwaitComposerSettle observes only input paint; native work may keep animating
// the rest of the pane. The submit hook remains the authoritative byte witness.
func AwaitComposerSettle(ctx context.Context, capture captureScreen, pane substrate.PaneID, collar Collar, settle time.Duration) error {
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	observed := composerStability{}
	for {
		screen, err := capture(ctx, pane)
		if err != nil {
			return err
		}
		composer, err := ReadComposer(collar.Primitives.Composer, screen)
		if err != nil {
			return err
		}
		if observed.ready(composer.Text, time.Now(), settle) {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("native composer did not settle before submission: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}

type composerStability struct {
	text  string
	since time.Time
}

func (observed *composerStability) ready(text string, now time.Time, settle time.Duration) bool {
	if text == "" {
		observed.text = ""
		observed.since = time.Time{}
		return false
	}
	if text != observed.text || observed.since.IsZero() {
		observed.text = text
		observed.since = now
	}
	return now.Sub(observed.since) >= settle
}
