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
// A clipped or foreign composer is refused once it has read so in every frame
// for settle, and in at least two frames (see heldRefusal).
func AwaitOwnedComposerSettle(ctx context.Context, capture captureScreen, pane substrate.PaneID, collar Collar, owner string, settle time.Duration) error {
	ticker := time.NewTicker(composerFrame)
	defer ticker.Stop()
	observed := composerStability{}
	refused := heldRefusal{}
	var lastErr error
	unsettled := func(ctxErr error) error {
		return fmt.Errorf("native composer did not settle before submission: %w", errors.Join(ctxErr, lastErr))
	}
	for {
		screen, err := capture(ctx, pane)
		if err != nil {
			// A capture cut short by the operation deadline, whether it
			// reports the context or the killed tmux, says nothing about
			// the pane; the composer never settled in time.
			if ctxErr := ctx.Err(); ctxErr != nil {
				return unsettled(ctxErr)
			}
			return err
		}
		composer, err := ReadOwnedComposer(collar.Primitives.Composer, screen, owner)
		now := time.Now()
		if held := refused.held(err, now, settle); held != nil {
			return held
		}
		if err != nil && !refusal(err) && !errors.Is(err, ErrNoComposer) && !errors.Is(err, ErrComposerOwnerHidden) {
			return err
		}
		lastErr = err
		// A missing frame need not mean the pasted input was lost. Require
		// a fresh stability window when it returns; never submit a missing
		// composer or retry input that has already been pasted. A paste hint
		// without a matching recorded owner needs the visible session footer.
		if err != nil {
			observed.text, observed.since, observed.emptySince = "", time.Time{}, time.Time{}
		} else if observed.ready(composer.Text, now, settle) {
			return nil
		} else if observed.emptied(composer.Text, now, settle) {
			return ErrComposerEmptied
		}
		select {
		case <-ctx.Done():
			return unsettled(ctx.Err())
		case <-ticker.C:
		}
	}
}

// composerFrame is how often the waits before the submit key read the pane.
const composerFrame = 25 * time.Millisecond

// AwaitPasteShown waits between a paste and its submit key for a collar
// that pauses before submitting (see SubmitPauses). It returns once settle
// has passed since it began and the screen shows the paste (see PasteShown).
// A paste that showed and then read empty for a settle window returns
// ErrComposerEmptied. A composer that holds other text in every frame of a
// settle window, and in at least two frames, returns ErrComposerReplaced;
// an unreadable frame restarts that window. A deadline reached when the
// last readable composer held other text also returns ErrComposerReplaced.
// That text is neither submitted nor cleared. A composer of another session
// is refused once it has read so in every frame for settle, and in at least
// two frames (see heldRefusal); one that still reads so at the deadline is
// the error, unless other text was read first.
func AwaitPasteShown(ctx context.Context, capture captureScreen, pane substrate.PaneID, collar Collar, wire, owner string, settle time.Duration) error {
	ticker := time.NewTicker(composerFrame)
	defer ticker.Stop()
	start := time.Now()
	var painted, replaced bool
	var emptySince, replacedSince time.Time
	refused := heldRefusal{}
	// Other text comes first: its error keeps the keys that withdraw a
	// paste off what may be another author's draft.
	expired := func(err error) error {
		if replaced {
			return fmt.Errorf("%w: %w", ErrComposerReplaced, err)
		}
		if refused.err != nil {
			return fmt.Errorf("%w: %w", refused.err, err)
		}
		return fmt.Errorf("pasted input did not appear in the composer: %w", err)
	}
	for {
		screen, err := capture(ctx, pane)
		if err != nil {
			// A capture cut short by the operation deadline says nothing
			// about the pane; the paste never showed in time.
			if ctxErr := ctx.Err(); ctxErr != nil {
				return expired(ctxErr)
			}
			return fmt.Errorf("read the composer after the paste: %w", err)
		}
		view, err := readPaste(collar, screen, wire, owner)
		now := time.Now()
		if held := refused.held(err, now, settle); held != nil {
			return held
		}
		if err != nil && !refusal(err) {
			return err
		}
		if view != pasteEmpty {
			emptySince = time.Time{}
		}
		// The other text must read for a continuous settle window; the
		// last readable composer decides the error at the deadline.
		if view != pasteReplaced {
			replacedSince = time.Time{}
		}
		if view == pasteShown || view == pasteEmpty {
			replaced = false
		}
		switch view {
		case pasteShown:
			if now.Sub(start) >= settle {
				return nil
			}
			painted = true
		case pasteEmpty:
			if painted && emptySince.IsZero() {
				emptySince = now
			}
			if painted && now.Sub(emptySince) >= settle {
				return ErrComposerEmptied
			}
		case pasteReplaced:
			replaced = true
			if replacedSince.IsZero() {
				replacedSince = now
			}
			// At least two frames, even with no settle.
			if now.Sub(replacedSince) >= max(settle, composerFrame) {
				return ErrComposerReplaced
			}
		}
		select {
		case <-ctx.Done():
			return expired(ctx.Err())
		case <-ticker.C:
		}
	}
}

// PasteShown reads a screen captured after a paste into an empty composer
// that a read before the paste found owned by owner. A composer of that owner
// shows the paste when its text begins with the start of wire, the text
// pasted, or with the placeholder Claude Code shows for a long paste, or is
// the end of wire from its closing tag (see endsWire); the submit hook
// confirms what is submitted. A short pane can clip the composer
// or its footer so that it cannot be read in full. The paste is then shown
// when a prompt line begins with the start of wire directly under a rule
// that carries owner, or, for a paste that pushes that rule above the pane,
// on the top row above the main conversation's footer. It returns an error on
// evidence that
// the submit key would act on something else: another surface owning input,
// a background-session composer, or a composer of another session. Otherwise
// false means not shown yet.
func PasteShown(collar Collar, screen substrate.Screen, wire, owner string) (bool, error) {
	view, err := readPaste(collar, screen, wire, owner)
	return view == pasteShown, err
}

type pasteView int

const (
	pasteUnseen pasteView = iota
	pasteShown
	pasteEmpty
	// pasteReplaced is a composer holding text that neither begins with
	// the paste nor is its end, such as a draft typed after the paste was
	// cleared.
	pasteReplaced
)

func readPaste(collar Collar, screen substrate.Screen, wire, owner string) (pasteView, error) {
	composer, err := ReadOwnedComposer(collar.Primitives.Composer, screen, owner)
	switch {
	case err == nil:
		switch {
		case composer.Text == "":
			return pasteEmpty, nil
		case beginsPaste(composer.Text, wire), endsWire(composer.Text, wire):
			return pasteShown, nil
		}
		return pasteReplaced, nil
	case errors.Is(err, ErrComposerOwnerHidden):
		// The paste hint expires and the session footer returns.
		return pasteUnseen, nil
	case errors.Is(err, ErrComposerOwnerClipped):
		return pasteUnseen, fmt.Errorf("%w: its rule names %q, not %q", ErrForeignComposer, composer.Owner, owner)
	case errors.Is(err, ErrComposerClipped), errors.Is(err, ErrNoComposer):
		if pasteStartShown(collar.Primitives.Composer, screen, wire, owner) {
			return pasteShown, nil
		}
		return pasteUnseen, nil
	}
	return pasteUnseen, err
}

// longPastePlaceholder is the start of the placeholder Claude Code shows in
// place of a long text paste.
var longPastePlaceholder = regexp.MustCompile(`^\[(?:Pasted text|\.\.\.Truncated text) #\d+`)

// beginsPaste reports text whose first row is the start of wire, ignoring
// the whitespace that wrapping changes, or begins with a long-paste
// placeholder.
func beginsPaste(text, wire string) bool {
	first, _, _ := strings.Cut(text, "\n")
	return longPastePlaceholder.MatchString(first) || beginsWire(first, wire)
}

// beginsWire reports a row that is the start of wire, ignoring the
// whitespace that wrapping changes.
func beginsWire(row, wire string) bool {
	start := strings.Join(strings.Fields(row), "")
	return start != "" && strings.HasPrefix(strings.Join(strings.Fields(wire), ""), start)
}

// endsWire reports composer text that is the end of wire from at least its
// closing tag, ignoring the whitespace that wrapping changes. Claude Code
// scrolls a composer taller than it allows so that its start lies above the
// prompt line; the closing tag carries the one-time token, so no draft or
// earlier message ends the same way.
func endsWire(text, wire string) bool {
	closing := strings.LastIndex(wire, "[/")
	if closing < 0 {
		return false
	}
	joined := strings.Join(strings.Fields(text), "")
	tail := strings.Join(strings.Fields(wire[closing:]), "")
	return strings.HasSuffix(joined, tail) && strings.HasSuffix(strings.Join(strings.Fields(wire), ""), joined)
}

// pasteStartShown reports a prompt line that begins with the start of wire
// directly under a composer rule labelled owner, or a paste that fills the
// pane (tallPasteShown). Before submission no transcript row repeats the start
// of this message, with its one-time token.
func pasteStartShown(invocation Invocation, screen substrate.Screen, wire, owner string) bool {
	if invocation.Name != "claude-composer" {
		return false
	}
	lines := screenLines(screen, false)
	if tallPasteShown(lines, wire) {
		return true
	}
	for index := len(lines) - 1; index > 0; index-- {
		line := strings.TrimSpace(lines[index])
		// A placeholder carries no one-time token, so it may be an earlier
		// paste in the transcript.
		if !strings.HasPrefix(line, "❯") || !beginsWire(strings.TrimPrefix(line, "❯"), wire) {
			continue
		}
		rule := strings.TrimSpace(lines[index-1])
		if strings.HasPrefix(rule, "─") && strings.HasSuffix(rule, "─") && strings.Trim(rule, "─ ") == owner {
			return true
		}
	}
	return false
}

// tallPasteShown reports a paste taller than the pane leaves room for: its
// prompt line, beginning with the start of wire, is the top row, so the
// composer's opening rule and the label it may carry lie above the pane. Every
// row below it up to the closing rule continues the input, and the footer
// below that rule shows the main conversation's mode line with no session list
// selecting another session, which a child session's composer does not show.
// The paste hint that replaces the mode line for a moment after a paste shows
// in a child's composer too, so the wait goes on until the mode line returns.
func tallPasteShown(lines []string, wire string) bool {
	if len(lines) == 0 || !strings.HasPrefix(lines[0], "❯") || !beginsWire(strings.TrimPrefix(lines[0], "❯"), wire) {
		return false
	}
	closing := lastRule(lines)
	if closing <= 0 {
		return false
	}
	for _, row := range lines[1:closing] {
		if strings.TrimSpace(row) != "" && !strings.HasPrefix(row, "  ") {
			return false
		}
	}
	mode, main := parentConversation(lines[closing+1:])
	return mode && main
}

// ErrComposerEmptied reports pasted input that left the composer before it
// was submitted: something other than gang cleared or consumed it.
var ErrComposerEmptied = errors.New("native composer emptied before submission")

// ErrComposerReplaced reports a composer that holds text that neither begins
// with the paste nor is its end: something other than gang replaced the paste, and the text
// may be another author's draft.
var ErrComposerReplaced = errors.New("native composer holds other text before submission")

// refusal reports a verdict that a composer is clipped or belongs to another
// session. A repaint that stalls partway can leave a torn screen that two
// captures agree on, and a torn screen reads as one of these.
func refusal(err error) bool {
	return errors.Is(err, ErrComposerClipped) || errors.Is(err, ErrComposerOwnerClipped) || errors.Is(err, ErrForeignComposer)
}

// heldRefusal holds a refusal back until it lasts.
type heldRefusal struct {
	err   error
	since time.Time
}

// held records the verdict err of a frame read at now. It returns a refusal
// once refusals have read in every frame for a settle window, and in at least
// two frames; until then a refusing frame reads as an unreadable one. Any
// other verdict, an unreadable frame included, ends the window, so isolated
// torn frames never add up to a refusal.
func (r *heldRefusal) held(err error, now time.Time, settle time.Duration) error {
	if !refusal(err) {
		r.err, r.since = nil, time.Time{}
		return nil
	}
	if r.err == nil {
		r.since = now
	}
	r.err = err
	if now.Sub(r.since) >= max(settle, composerFrame) {
		return err
	}
	return nil
}

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
