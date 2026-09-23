package main

import (
	"fmt"
	"strings"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/store"
	"github.com/adambiggs/gangline/substrate"
)

func (run *runtime) observeCompaction(l *store.LockedAgent, a *core.Agent, c harness.Collar, screen substrate.Screen) error {
	pending := a.Compaction
	if pending == nil || (pending.Status != "submitted" && pending.Status != "unverified") {
		return nil
	}
	if pending.CompletedAt.After(pending.StartedAt) {
		return run.continueCompaction(l, a)
	}
	refusals, err := harness.ActionRefusals(c.Actions.Compact, screen)
	if err != nil {
		return err
	}
	if len(refusals) > pending.RefusalBefore {
		reason := strings.TrimSpace(refusals[len(refusals)-1])
		if err := run.apply(l, a, core.Event{Type: "compaction_failed", ID: pending.ID, Reason: reason}); err != nil {
			return err
		}
		return commandError{status: exitNative, text: fmt.Sprintf("native compaction refused: %s; resume withheld", reason)}
	}
	return nil
}
