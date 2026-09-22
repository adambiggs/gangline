//go:build linux

package tmux

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type linuxProcessHandle struct {
	process *os.Process
}

func bootIdentity() (string, error) {
	data, err := os.ReadFile("/proc/sys/kernel/random/boot_id")
	return strings.TrimSpace(string(data)), err
}

func observeProcess(pid int) (processObservation, error) {
	stat, err := os.Open(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return processObservation{}, err
	}
	read := func() (processRecord, error) {
		if _, err := stat.Seek(0, io.SeekStart); err != nil {
			return processRecord{}, err
		}
		data, err := io.ReadAll(stat)
		if err != nil {
			return processRecord{}, err
		}
		return parseProcStat(string(data))
	}
	record, err := read()
	if err != nil {
		return processObservation{}, errors.Join(err, stat.Close())
	}
	return processObservation{record: record, read: read, close: stat.Close}, nil
}

func openProcessHandle(observed processRecord) (processHandle, error) {
	process, err := os.FindProcess(observed.PID)
	if err != nil {
		return nil, err
	}
	// FindProcess can fall back to numeric signalling on old kernels.
	// Refuse that fallback before tmux or any signal can have an effect.
	if err := process.WithHandle(func(uintptr) {}); err != nil {
		_ = process.Release()
		return nil, fmt.Errorf("identity-bound process handle unavailable: %w", err)
	}
	return &linuxProcessHandle{process: process}, nil
}

func parseProcStat(data string) (processRecord, error) {
	var record processRecord
	left, right := strings.IndexByte(data, '('), strings.LastIndexByte(data, ')')
	if left <= 0 || right <= left {
		return record, fmt.Errorf("read process identity: malformed proc stat")
	}
	pid, err := strconv.Atoi(strings.TrimSpace(data[:left]))
	if err != nil || pid <= 0 {
		return record, fmt.Errorf("read process identity: invalid PID")
	}
	fields := strings.Fields(data[right+1:])
	if len(fields) < 20 {
		return record, fmt.Errorf("read process identity: proc stat has %d trailing fields", len(fields))
	}
	parent, err := strconv.Atoi(fields[1])
	if err != nil {
		return record, fmt.Errorf("read process identity: parent PID: %w", err)
	}
	if _, err := strconv.ParseUint(fields[19], 10, 64); err != nil {
		return record, fmt.Errorf("read process identity: start ticks: %w", err)
	}
	record.PID, record.ParentPID, record.started = pid, parent, fields[19]
	return record, nil
}

func (handle *linuxProcessHandle) signal(signal syscall.Signal) error {
	return handle.process.Signal(signal)
}

func (handle *linuxProcessHandle) close() error {
	return handle.process.Release()
}

func (handle *linuxProcessHandle) wait(ctx context.Context) error {
	var waitErr error
	err := handle.process.WithHandle(func(fd uintptr) { waitErr = waitPIDFD(ctx, int(fd)) })
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
		if events[0].Events&(syscall.EPOLLIN|syscall.EPOLLHUP) == 0 {
			return fmt.Errorf("unexpected process exit event: %#x", events[0].Events)
		}
		return nil
	}
}

func readCurrentProcess(pid int) (processRecord, error) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return processRecord{}, err
	}
	return parseProcStat(string(data))
}
