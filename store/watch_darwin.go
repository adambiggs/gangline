//go:build darwin

package store

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
)

func newFileWatch(path string) (<-chan error, func() error, error) {
	return newFileWatchKevent(path, syscall.Kevent)
}

func newFileWatchKevent(path string, kevent func(int, []syscall.Kevent_t, []syscall.Kevent_t, *syscall.Timespec) (int, error)) (<-chan error, func() error, error) {
	directory, err := os.Open(filepath.Dir(path))
	if err != nil {
		return nil, nil, fmt.Errorf("open state directory for watch: %w", err)
	}
	queue, err := syscall.Kqueue()
	if err != nil {
		_ = directory.Close()
		return nil, nil, fmt.Errorf("create state directory watch: %w", err)
	}
	change := syscall.Kevent_t{
		Ident:  uint64(directory.Fd()),
		Filter: syscall.EVFILT_VNODE,
		Flags:  syscall.EV_ADD | syscall.EV_CLEAR,
		Fflags: syscall.NOTE_WRITE | syscall.NOTE_EXTEND | syscall.NOTE_DELETE | syscall.NOTE_RENAME,
	}
	if err := retryInterrupted(func() error {
		_, err := kevent(queue, []syscall.Kevent_t{change}, nil, nil)
		return err
	}); err != nil {
		_ = syscall.Close(queue)
		_ = directory.Close()
		return nil, nil, fmt.Errorf("watch state directory: %w", err)
	}
	file, err := os.Open(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		_ = syscall.Close(queue)
		_ = directory.Close()
		return nil, nil, fmt.Errorf("open state file for watch: %w", err)
	}
	if err == nil {
		change.Ident = uint64(file.Fd())
		if err := retryInterrupted(func() error {
			_, err := kevent(queue, []syscall.Kevent_t{change}, nil, nil)
			return err
		}); err != nil {
			_ = syscall.Close(queue)
			_ = file.Close()
			_ = directory.Close()
			return nil, nil, fmt.Errorf("watch state file: %w", err)
		}
	}
	events := make(chan error, 1)
	go func() {
		ready := make([]syscall.Kevent_t, 1)
		err := retryInterrupted(func() error {
			_, err := kevent(queue, nil, ready, nil)
			return err
		})
		events <- err
	}()
	var once sync.Once
	var closeErr error
	closeWatch := func() error {
		once.Do(func() {
			queueErr := syscall.Close(queue)
			var fileErr error
			if file != nil {
				fileErr = file.Close()
			}
			directoryErr := directory.Close()
			for _, err := range []error{queueErr, fileErr, directoryErr} {
				if closeErr == nil {
					closeErr = err
				}
			}
		})
		return closeErr
	}
	return events, closeWatch, nil
}
