package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"unicode/utf8"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/store"
	"github.com/adambiggs/gangline/substrate"
)

func startupWitnessUnchanged(paths store.AgentPaths, oldID string) error {
	witness, err := paths.ReadWitness()
	if errors.Is(err, os.ErrNotExist) {
		if oldID == "" {
			return nil
		}
		return fmt.Errorf("startup submit witness disappeared during recovery")
	}
	if err != nil {
		return err
	}
	if witness.ID != oldID {
		return fmt.Errorf("startup submit witness changed during recovery")
	}
	return nil
}

// Persist the loss of negative proof before any Enter can reach the harness.
func startupSubmitPossible(paths store.AgentPaths, e *core.Envelope) error {
	if e.PasteOnly == nil {
		return nil
	}
	e.PasteOnly = nil
	if err := paths.Publish(*e); err != nil {
		return err
	}
	path, err := paths.EnvelopePath("new", e.ID)
	if err != nil {
		return err
	}
	// The old paste-only record must not reappear after a host crash.
	failedPath, err := paths.EnvelopePath("failed", e.ID)
	if err != nil {
		return err
	}
	for _, name := range []string{path, filepath.Dir(path), filepath.Dir(failedPath)} {
		f, err := os.Open(name)
		if err != nil {
			return err
		}
		if err := errors.Join(f.Sync(), f.Close()); err != nil {
			return err
		}
	}
	return nil
}

func (run *runtime) replaceCollapsedStartup(ctx context.Context, b harnessInput, pane substrate.PaneID, c harness.Collar, wire string, paths store.AgentPaths, oldID string, clear bool, beforeSubmit func() error) error {
	wantChars := utf8.RuneCountInString(wire)
	check := func(wantCollapsed bool) error {
		screen, err := b.Capture(ctx, pane)
		if err != nil {
			return err
		}
		if blocked, found, err := harness.InputBlocked(c, screen); err != nil {
			return err
		} else if found {
			return fmt.Errorf("native prompt owns input during startup recovery: %s", blocked.Evidence)
		}
		composer, err := harness.ReadComposer(c.Primitives.Composer, screen)
		if err != nil {
			return err
		}
		if wantCollapsed {
			if composer.CollapsedChars != wantChars && !harness.SameComposerText(composer.Text, wire) {
				return fmt.Errorf("collapsed startup draft changed during recovery")
			}
		} else {
			idle, err := harness.Idle(c, screen)
			if err != nil {
				return err
			}
			if composer.Text != "" || composer.TailOccupied || !idle {
				return fmt.Errorf("startup composer is not empty after clear")
			}
		}
		return startupWitnessUnchanged(paths, oldID)
	}
	if clear {
		if err := beforeSubmit(); err != nil {
			return err
		}
		if err := check(true); err != nil {
			return err
		}
		if err := sendHarnessKeys(ctx, b, pane, c, c.Actions.StartupReplace.Input()); err != nil {
			return err
		}
	}
	if err := check(false); err != nil {
		return err
	}
	input, err := harness.SubmitInput(c.Primitives.Submit, wire)
	if err != nil {
		return err
	}
	if !clear && !startupPasteSafe(input, wire) {
		return fmt.Errorf("empty startup recovery requires bracketed paste")
	}
	if err := sendHarnessKeys(ctx, b, pane, c, input); err != nil {
		return err
	}
	settle, err := harness.SubmitSettle(c.Primitives.Submit)
	if err != nil {
		return err
	}
	if run.cmd.settleInput != nil {
		err = run.cmd.settleInput(ctx, b, pane, c, settle)
	} else {
		err = harness.AwaitComposerSettle(ctx, b.Capture, pane, c, settle)
	}
	if err != nil {
		return err
	}
	if err := check(true); err != nil {
		return err
	}
	if err := beforeSubmit(); err != nil {
		return err
	}
	return sendHarnessKeys(ctx, b, pane, c, substrate.Keys{Submit: true})
}
