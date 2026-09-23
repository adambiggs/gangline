package main

import (
	"context"
	"errors"
	"os"
	"time"

	"github.com/adambiggs/gangline/store"
)

type changeWait interface {
	Wait(context.Context) error
	Close() error
}

func (cmd command) watch(path string) (changeWait, error) {
	if cmd.newWatch != nil {
		return cmd.newWatch(path)
	}
	return store.Watch(path)
}
func (cmd command) awaitWitness(ctx context.Context, p store.AgentPaths, before string) (store.Witness, error) {
	witness, _, err := cmd.awaitReceipt(ctx, p, before, nil)
	return witness, err
}

// A queue receipt has no filesystem notification. While this one submission is
// pending, bounded screen observations share the native witness wait.
func (cmd command) awaitReceipt(ctx context.Context, p store.AgentPaths, before string, queue func(context.Context) (bool, error)) (store.Witness, bool, error) {
	for {
		watch, err := cmd.watch(p.Witness)
		if err != nil {
			return store.Witness{}, false, err
		}
		witness, err := p.ReadWitness()
		if err == nil && witness.ID != before {
			_ = watch.Close()
			return witness, false, nil
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			_ = watch.Close()
			return witness, false, err
		}
		waitCtx := ctx
		cancel := func() {}
		if queue != nil {
			accepted, queueErr := queue(ctx)
			if queueErr != nil || accepted {
				closeErr := watch.Close()
				return store.Witness{}, accepted, errors.Join(queueErr, closeErr)
			}
			waitCtx, cancel = context.WithTimeout(ctx, 100*time.Millisecond)
		}
		err = watch.Wait(waitCtx)
		cancel()
		closeErr := watch.Close()
		expired := ctx.Err() != nil
		if deadline, ok := ctx.Deadline(); ok && !cmd.now().Before(deadline) {
			expired = true
		}
		if queue != nil && errors.Is(err, context.DeadlineExceeded) && !expired {
			err = nil
		}
		if err != nil {
			return witness, false, err
		}
		if closeErr != nil {
			return witness, false, closeErr
		}
	}
}
