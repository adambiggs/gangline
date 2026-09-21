//go:build darwin

package tmux

import (
	"context"
	"errors"
	"syscall"
	"time"
)

func waitProcessExit(ctx context.Context, pid int) error {
	kqueue, err := syscall.Kqueue()
	if err != nil {
		return err
	}
	defer syscall.Close(kqueue)
	change := syscall.Kevent_t{
		Ident: uint64(pid), Filter: syscall.EVFILT_PROC,
		Flags: uint16(syscall.EV_ADD | syscall.EV_ONESHOT), Fflags: uint32(syscall.NOTE_EXIT),
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	var timeout *syscall.Timespec
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return context.DeadlineExceeded
		}
		value := syscall.NsecToTimespec(remaining.Nanoseconds())
		timeout = &value
	}
	events := make([]syscall.Kevent_t, 1)
	count, err := syscall.Kevent(kqueue, []syscall.Kevent_t{change}, events, timeout)
	if errors.Is(err, syscall.ESRCH) || errors.Is(err, syscall.ENOENT) {
		return nil
	}
	if err != nil {
		return err
	}
	if count == 0 {
		return context.DeadlineExceeded
	}
	if events[0].Flags&syscall.EV_ERROR != 0 {
		eventErr := syscall.Errno(events[0].Data)
		if errors.Is(eventErr, syscall.ESRCH) || errors.Is(eventErr, syscall.ENOENT) {
			return nil
		}
		return eventErr
	}
	return nil
}
