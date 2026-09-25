package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"unicode/utf8"

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

func (run *runtime) replaceCollapsedStartup(ctx context.Context, b harnessInput, pane substrate.PaneID, c harness.Collar, wire string, paths store.AgentPaths, oldID string) error {
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
			if composer.CollapsedChars != wantChars {
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
	if err := check(true); err != nil {
		return err
	}
	if err := sendHarnessKeys(ctx, b, pane, c, c.Actions.StartupReplace.Input()); err != nil {
		return err
	}
	if err := check(false); err != nil {
		return err
	}
	input, err := harness.SubmitInput(c.Primitives.Submit, wire)
	if err != nil {
		return err
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
	return sendHarnessKeys(ctx, b, pane, c, substrate.Keys{Submit: true})
}
