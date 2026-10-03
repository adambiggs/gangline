package harness

import (
	"context"
	"errors"
	"fmt"
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
	busy, err := Busy(collar, screen)
	return composer.Text == "" && !busy, err
}

// Busy checks native activity even while an action occupies the composer.
func Busy(collar Collar, screen substrate.Screen) (bool, error) {
	pattern := collar.Primitives.Wedge.Params["busy"]
	if pattern == "" {
		return false, fmt.Errorf("collar has no native busy expression")
	}
	busy, err := regexp.Compile(pattern)
	if err != nil {
		return false, err
	}
	return busy.MatchString(strings.Join(screenLines(screen, true), "\n")), nil
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
		if readErr == nil && SameComposerText(composer.Text, want) {
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
	readySettle := 400 * time.Millisecond
	if collar.Name == "codex" {
		// Codex can paint an empty composer before its folder or hook trust menu.
		readySettle = 3 * time.Second
	}
	deadline := time.NewTimer(8 * time.Second)
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
			return Startup{}, substrate.Screen{}, fmt.Errorf("native startup was not observable within 8s: %w", lastErr)
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
	var lastErr error
	for {
		screen, err := capture(ctx, pane)
		if err != nil {
			// A capture cut short by the operation deadline, whether it
			// reports the context or the killed tmux, says nothing about
			// the pane; the composer never settled in time.
			if ctxErr := ctx.Err(); ctxErr != nil {
				return fmt.Errorf("native composer did not settle before submission: %w", errors.Join(ctxErr, lastErr))
			}
			return err
		}
		composer, err := ReadComposer(collar.Primitives.Composer, screen)
		if err != nil && !errors.Is(err, ErrNoComposer) && !errors.Is(err, ErrComposerOwnerHidden) {
			return err
		}
		lastErr = err
		// A missing frame need not mean the pasted input was lost. Require
		// a fresh stability window when it returns; never submit a missing
		// composer or retry input that has already been pasted. A paste hint
		// hides the composer's owner until it expires, and the owner is
		// checked again on the frame that follows.
		if err != nil {
			observed.text, observed.since, observed.emptySince = "", time.Time{}, time.Time{}
		} else if observed.ready(composer.Text, time.Now(), settle) {
			return nil
		} else if observed.emptied(composer.Text, time.Now(), settle) {
			return ErrComposerEmptied
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("native composer did not settle before submission: %w", errors.Join(ctx.Err(), lastErr))
		case <-ticker.C:
		}
	}
}

// ErrComposerEmptied reports pasted input that left the composer before it
// was submitted: something other than gang cleared or consumed it.
var ErrComposerEmptied = errors.New("native composer emptied before submission")

type composerStability struct {
	text  string
	since time.Time
	// painted records that the composer has shown text; emptySince when it
	// last read empty afterwards.
	painted    bool
	emptySince time.Time
}

// emptied reports a composer that showed text and has read empty for a full
// settle window since.
func (observed *composerStability) emptied(text string, now time.Time, settle time.Duration) bool {
	if text != "" {
		observed.painted, observed.emptySince = true, time.Time{}
		return false
	}
	if !observed.painted {
		return false
	}
	if observed.emptySince.IsZero() {
		observed.emptySince = now
	}
	return now.Sub(observed.emptySince) >= settle
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
