//go:build linux

package store

import (
	"fmt"
	"os"
	"sync"
	"syscall"
)

func newFileWatch(path string) (<-chan error, func() error, error) {
	fd, err := syscall.InotifyInit1(syscall.IN_CLOEXEC | syscall.IN_NONBLOCK)
	if err != nil {
		return nil, nil, fmt.Errorf("create state directory watch: %w", err)
	}
	file := os.NewFile(uintptr(fd), "state-directory-watch")
	if _, err := syscall.InotifyAddWatch(fd, path, syscall.IN_MODIFY|syscall.IN_CLOSE_WRITE|syscall.IN_DELETE_SELF|syscall.IN_MOVE_SELF|syscall.IN_MOVED_TO|syscall.IN_CREATE|syscall.IN_DELETE); err != nil {
		_ = file.Close()
		return nil, nil, fmt.Errorf("watch state directory: %w", err)
	}
	events := make(chan error, 1)
	go func() {
		buffer := make([]byte, syscall.SizeofInotifyEvent*8)
		_, err := file.Read(buffer)
		events <- err
	}()
	var once sync.Once
	var closeErr error
	closeWatch := func() error {
		once.Do(func() { closeErr = file.Close() })
		return closeErr
	}
	return events, closeWatch, nil
}
