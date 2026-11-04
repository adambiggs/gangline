package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
	"github.com/adambiggs/gangline/store"
	"github.com/adambiggs/gangline/substrate"
)

type pendingInputError struct{ reason string }

func (e *pendingInputError) Error() string { return e.reason }
func (e *pendingInputError) Unwrap() error { return harness.ErrNoComposer }

// pendingInput is called only by pre-paste observations, never by a submit or
// settle error. Missing input cannot establish foreign foreground ownership.
func (run *runtime) pendingInput(l *store.LockedAgent, a *core.Agent, b harnessInput, c harness.Collar, err error) error {
	if err != harness.ErrNoComposer || a.Input != nil {
		return err
	}
	if foregroundErr := requireHarnessForeground(context.Background(), b, substrate.PaneID(a.Pane), c); foregroundErr != nil {
		return errors.Join(err, foregroundErr)
	}
	queued, readErr := l.Paths.ListNew()
	if readErr != nil {
		return errors.Join(err, readErr)
	}
	if len(queued) == 0 {
		return err
	}
	reason := fmt.Sprintf("%s pending input could not be observed: %v; message remains queued; inspect the native pane", a.Name, err)
	event := ""
	if a.InputOutage == nil {
		a.InputOutage = &core.InputOutage{Since: run.cmd.now()}
		event = "input_pending"
	} else if !a.InputOutage.Escalated && !run.cmd.now().Before(a.InputOutage.Since.Add(operationTimeout)) {
		a.InputOutage.Escalated = true
		event = "input_observation_outage"
		reason = fmt.Sprintf("%s persistent input observation outage since %s; message remains queued; inspect the native pane", a.Name, a.InputOutage.Since.UTC().Format("2006-01-02T15:04:05Z07:00"))
	}
	if event != "" {
		if recordErr := run.record(*a, core.Event{Type: event, Reason: reason}); recordErr != nil {
			return errors.Join(err, recordErr)
		}
		if saveErr := l.Save(*a); saveErr != nil {
			return errors.Join(err, saveErr)
		}
	}
	return &pendingInputError{reason: reason}
}

func (run *runtime) clearInputOutage(l *store.LockedAgent, a *core.Agent) error {
	if a.InputOutage == nil {
		return nil
	}
	if err := run.record(*a, core.Event{Type: "input_observation_recovered", Reason: "registered native composer observed again"}); err != nil {
		return err
	}
	a.InputOutage = nil
	return l.Save(*a)
}
