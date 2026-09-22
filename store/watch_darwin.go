//go:build darwin

package store

import (
	"fmt"
	"os"
	"sync"
	"syscall"
)

func newFileWatch(path string) (<-chan error, func() error, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, nil, fmt.Errorf("open state directory for watch: %w", err)
	}
	queue, err := syscall.Kqueue()
	if err != nil {
		_ = file.Close()
		return nil, nil, fmt.Errorf("create state directory watch: %w", err)
	}
	change := syscall.Kevent_t{
		Ident:  uint64(file.Fd()),
		Filter: syscall.EVFILT_VNODE,
		Flags:  syscall.EV_ADD | syscall.EV_CLEAR,
		Fflags: syscall.NOTE_WRITE | syscall.NOTE_EXTEND | syscall.NOTE_DELETE | syscall.NOTE_RENAME,
	}
	if _, err := syscall.Kevent(queue, []syscall.Kevent_t{change}, nil, nil); err != nil {
		_ = syscall.Close(queue)
		_ = file.Close()
		return nil, nil, fmt.Errorf("watch state directory: %w", err)
	}
	events := make(chan error, 1)
	go func() {
		_, err := syscall.Kevent(queue, nil, make([]syscall.Kevent_t, 1), nil)
		events <- err
	}()
	var once sync.Once
	var closeErr error
	closeWatch := func() error {
		once.Do(func() {
			queueErr := syscall.Close(queue)
			fileErr := file.Close()
			if queueErr != nil {
				closeErr = queueErr
			} else {
				closeErr = fileErr
			}
		})
		return closeErr
	}
	return events, closeWatch, nil
}
