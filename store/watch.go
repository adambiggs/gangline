package store

import (
	"context"
	"errors"
	"path/filepath"
	"syscall"
)

func retryInterrupted(call func() error) error {
	for {
		err := call()
		if !errors.Is(err, syscall.EINTR) {
			return err
		}
	}
}

type ChangeWait struct {
	events <-chan error
	close  func() error
}

// Watch registers the parent directory. Read the target after registration,
// then wait: an atomic replacement cannot fall between registration and read.
func Watch(path string) (*ChangeWait, error) {
	events, closeWatch, err := newFileWatch(filepath.Dir(path))
	if err != nil {
		return nil, err
	}
	return &ChangeWait{events, closeWatch}, nil
}
func (w *ChangeWait) Wait(ctx context.Context) error {
	select {
	case err := <-w.events:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}
func (w *ChangeWait) Close() error {
	if w == nil || w.close == nil {
		return nil
	}
	f := w.close
	w.close = nil
	return f()
}
