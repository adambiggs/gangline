//go:build linux

package tmux

import (
	"context"
	"errors"
	"os"
	"syscall"
	"time"
)

func waitProcessExit(ctx context.Context, pid int) error {
	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	var waitErr error
	err = process.WithHandle(func(handle uintptr) {
		waitErr = waitPIDFD(ctx, int(handle))
	})
	if errors.Is(err, os.ErrNoHandle) {
		return nil
	}
	if err != nil {
		return err
	}
	return waitErr
}

func waitPIDFD(ctx context.Context, pidfd int) error {
	epoll, err := syscall.EpollCreate1(syscall.EPOLL_CLOEXEC)
	if err != nil {
		return err
	}
	defer syscall.Close(epoll)
	event := syscall.EpollEvent{Events: syscall.EPOLLIN, Fd: int32(pidfd)}
	if err := syscall.EpollCtl(epoll, syscall.EPOLL_CTL_ADD, pidfd, &event); err != nil {
		return err
	}

	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		timeout := -1
		if deadline, ok := ctx.Deadline(); ok {
			remaining := time.Until(deadline)
			if remaining <= 0 {
				return context.DeadlineExceeded
			}
			timeout = int((remaining + time.Millisecond - 1) / time.Millisecond)
		}
		events := make([]syscall.EpollEvent, 1)
		count, err := syscall.EpollWait(epoll, events, timeout)
		if errors.Is(err, syscall.EINTR) {
			continue
		}
		if err != nil {
			return err
		}
		if count == 0 {
			return context.DeadlineExceeded
		}
		return nil
	}
}
