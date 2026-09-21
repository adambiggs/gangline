package store

import (
	"context"
	"fmt"
	"os"
)

type AppendWait struct {
	events  <-chan error
	close   func() error
	changed bool
}

func NewAppendWait(path string, after int64) (*AppendWait, error) {
	if after < 0 {
		return nil, fmt.Errorf("event log size is negative")
	}
	events, closeWatch, err := newFileWatch(path)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(path)
	if err != nil {
		_ = closeWatch()
		return nil, fmt.Errorf("stat event log after watch registration: %w", err)
	}
	if info.Size() != after {
		if err := closeWatch(); err != nil {
			return nil, err
		}
		return &AppendWait{changed: true}, nil
	}
	return &AppendWait{events: events, close: closeWatch}, nil
}

func (wait *AppendWait) Wait(ctx context.Context) error {
	if wait.changed {
		return nil
	}
	select {
	case err := <-wait.events:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (wait *AppendWait) Close() error {
	if wait == nil || wait.close == nil {
		return nil
	}
	closeWatch := wait.close
	wait.close = nil
	return closeWatch()
}
