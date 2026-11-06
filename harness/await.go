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
// A pane title that the collar's title_busy expression matches is busy too;
// a title it does not match adds nothing.
func Busy(collar Collar, screen substrate.Screen) (bool, error) {
	pattern := collar.Primitives.Wedge.Params["busy"]
	if pattern == "" {
		return false, fmt.Errorf("collar has no native busy expression")
	}
	busy, err := regexp.Compile(pattern)
	if err != nil {
		return false, err
	}
	if busy.MatchString(strings.Join(screenLines(screen, true), "\n")) {
		return true, nil
	}
	return titleMatches(collar.Primitives.Wedge, "title_busy", screen.Title)
}

// titlePattern compiles the collar's named pane-title expression; an absent
// expression is nil.
func titlePattern(invocation Invocation, name string) (*regexp.Regexp, error) {
	expression := invocation.Params[name]
	if expression == "" {
		return nil, nil
	}
	pattern, err := regexp.Compile(expression)
	if err != nil {
		return nil, fmt.Errorf("%s primitive %s expression: %w", invocation.Name, name, err)
	}
	return pattern, nil
}

// titleMatches reports whether the collar's named title expression matches
// the pane title. An absent expression or an empty title matches nothing.
func titleMatches(invocation Invocation, name, title string) (bool, error) {
	pattern, err := titlePattern(invocation, name)
	if err != nil || pattern == nil || title == "" {
		return false, err
	}
	return pattern.MatchString(title), nil
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

// StartupUnobservedError reports a startup wait that ended while the screen
// read neither as ready nor as a recognized prompt. Last is the final reading.
type StartupUnobservedError struct {
	Last error
}

func (e *StartupUnobservedError) Error() string {
	return fmt.Sprintf("native startup was not observable within 8s: %v", e.Last)
}

func (e *StartupUnobservedError) Unwrap() error { return e.Last }

// AwaitStartup waits for an observable ready or trust-required startup state.
func AwaitStartup(ctx context.Context, capture captureScreen, pane substrate.PaneID, collar Collar) (Startup, substrate.Screen, error) {
	readySettle := 400 * time.Millisecond
	if collar.Primitives.Composer.Name == "codex-composer" {
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
			return Startup{}, substrate.Screen{}, &StartupUnobservedError{Last: lastErr}
		case <-ticker.C:
		}
	}
}

// AwaitComposerSettle observes only input paint; native work may keep animating
// the rest of the pane. The submit hook remains the authoritative byte witness.
func AwaitComposerSettle(ctx context.Context, capture captureScreen, pane substrate.PaneID, collar Collar, settle time.Duration) error {
	return AwaitOwnedComposerSettle(ctx, capture, pane, collar, "", settle)
}

// AwaitOwnedComposerSettle is AwaitComposerSettle for input typed into a
// composer that a read before typing found owned by owner. A frame whose
// footer the input clipped or replaced with a paste hint is that composer's
// while its rule still carries owner; an empty owner accepts no such frame.
func AwaitOwnedComposerSettle(ctx context.Context, capture captureScreen, pane substrate.PaneID, collar Collar, owner string, settle time.Duration) error {
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
		composer, err := ReadOwnedComposer(collar.Primitives.Composer, screen, owner)
		if err != nil && !errors.Is(err, ErrNoComposer) && !errors.Is(err, ErrComposerOwnerHidden) {
			return err
		}
		lastErr = err
		// A missing frame need not mean the pasted input was lost. Require
		// a fresh stability window when it returns; never submit a missing
		// composer or retry input that has already been pasted. A paste hint
		// without a matching recorded owner needs the visible session footer.
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

// PasteShown reads a screen captured after the pause that precedes a paste's
// submit key (see SubmitPauses). The composer read empty before the paste and
// was owned by owner, so a composer of that owner holding text holds the
// paste; the submit hook confirms what is submitted. A short pane can clip the
// composer or its footer so that it cannot be read in full. The paste is then
// shown when a prompt line begins with the start of wire, the text pasted,
// directly under a rule that carries owner, the label read before the paste.
// It returns an error on evidence that the submit key would act on something
// else: another surface owning input, a background-session composer, or a
// composer of another session. Otherwise false means not shown yet.
func PasteShown(collar Collar, screen substrate.Screen, wire, owner string) (bool, error) {
	composer, err := ReadOwnedComposer(collar.Primitives.Composer, screen, owner)
	switch {
	case err == nil:
		return composer.Text != "", nil
	case errors.Is(err, ErrComposerOwnerHidden):
		// The paste hint expires and the session footer returns.
		return false, nil
	case errors.Is(err, ErrComposerOwnerClipped):
		return false, fmt.Errorf("%w: its rule names %q, not %q", ErrForeignComposer, composer.Owner, owner)
	case errors.Is(err, ErrComposerClipped), errors.Is(err, ErrNoComposer):
		return pasteStartShown(collar.Primitives.Composer, screen, wire, owner), nil
	}
	return false, err
}

// pasteStartShown reports a prompt line that begins with the start of wire
// directly under a composer rule labelled owner.
func pasteStartShown(invocation Invocation, screen substrate.Screen, wire, owner string) bool {
	if invocation.Name != "claude-composer" {
		return false
	}
	pasted := strings.Join(strings.Fields(wire), "")
	lines := screenLines(screen, false)
	for index := len(lines) - 1; index > 0; index-- {
		line := strings.TrimSpace(lines[index])
		start := strings.Join(strings.Fields(strings.TrimPrefix(line, "❯")), "")
		if !strings.HasPrefix(line, "❯") || start == "" || !strings.HasPrefix(pasted, start) {
			continue
		}
		rule := strings.TrimSpace(lines[index-1])
		if strings.HasPrefix(rule, "─") && strings.HasSuffix(rule, "─") && strings.Trim(rule, "─ ") == owner {
			return true
		}
	}
	return false
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
