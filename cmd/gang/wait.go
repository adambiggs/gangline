package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/store"
)

func (cmd command) wait(arguments []string) error {
	options, err := parseWait(arguments)
	if err != nil {
		return err
	}
	run, err := cmd.runtime()
	if err != nil {
		return err
	}
	state, err := run.load()
	if err != nil {
		return err
	}
	hitch, ok := activeByName(state, options.Name)
	if !ok {
		return refuseError("agent %q is not active", options.Name)
	}
	if hitch.Activity == core.ActivityIdle {
		return nil
	}

	deadline := time.Now().Add(options.Timeout)
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()
	paths, err := run.paths().Team(run.settings.Session)
	if err != nil {
		return err
	}
	for {
		info, statErr := os.Stat(paths.Events)
		if statErr != nil {
			return fmt.Errorf("stat event log before wait: %w", statErr)
		}
		state, err = run.load()
		if err != nil {
			return err
		}
		hitch, ok = activeByName(state, options.Name)
		if !ok {
			return refuseError("agent %q is not active", options.Name)
		}
		if hitch.Activity == core.ActivityIdle {
			return nil
		}
		if !time.Now().Before(deadline) {
			return run.waitTimedOut(hitch, deadline)
		}
		appendWait, watchErr := cmd.appendWait(paths.Events, info.Size())
		if watchErr != nil {
			return watchErr
		}
		waitErr := appendWait.Wait(ctx)
		closeErr := appendWait.Close()
		if waitErr != nil {
			if errors.Is(waitErr, context.DeadlineExceeded) {
				return run.waitTimedOut(hitch, deadline)
			}
			return waitErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
}

type appendWait interface {
	Wait(context.Context) error
	Close() error
}

func (cmd command) appendWait(path string, after int64) (appendWait, error) {
	if cmd.newAppendWait != nil {
		return cmd.newAppendWait(path, after)
	}
	return store.NewAppendWait(path, after)
}

func (run *runtime) waitTimedOut(hitch core.Hitch, deadline time.Time) error {
	now := time.Now()
	_, err := run.drive(core.OperationTimedOut{
		At: now, Operation: core.TimeoutWait, ID: string(hitch.ID), Deadline: deadline,
		Evidence: "wait deadline elapsed before an idle boundary",
	})
	if err != nil {
		return err
	}
	return refuseError("agent %q did not reach an idle boundary before the wait deadline", hitch.Name)
}
