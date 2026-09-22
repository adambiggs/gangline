//go:build darwin

package tmux

import (
	"context"
	"errors"
	"fmt"
	"syscall"
	"time"
	"unsafe"
)

// XNU's proc_info ABI: PROC_INFO_CALL_PIDINFO,
// PROC_PIDUNIQIDENTIFIERINFO, and PROC_INFO_CALL_SIGNAL_AUDITTOKEN.
// The signal call checks PID and pidversion while holding the process ref.
const (
	procInfoPIDInfo      = 2
	procUniqueIDInfo     = 17
	procSignalAuditToken = 17
)

type darwinProcessInfo struct {
	UUID                           [16]byte
	UniqueID, ParentUniqueID       uint64
	Version, OriginalParentVersion uint32
	Reserved2, Reserved3           uint64
}

type darwinProcessHandle struct {
	pid     int
	started string
	token   [8]uint32
	kqueue  int
}

func bootIdentity() (string, error) {
	value, err := syscall.Sysctl("kern.boottime")
	return fmt.Sprintf("%x", value), err
}

func readDarwinProcess(pid int) (processRecord, uint32, error) {
	var info darwinProcessInfo
	size := unsafe.Sizeof(info)
	count, _, errno := syscall.Syscall6(syscall.SYS_PROC_INFO, procInfoPIDInfo, uintptr(pid), procUniqueIDInfo, 0, uintptr(unsafe.Pointer(&info)), size)
	if errno != 0 {
		return processRecord{}, 0, errno
	}
	if count != size || info.UniqueID == 0 {
		return processRecord{}, 0, fmt.Errorf("native process identity unavailable for %d: returned %d bytes", pid, count)
	}
	var record processRecord
	record.PID = pid
	record.uniqueID, record.parentUniqueID, record.version = info.UniqueID, info.ParentUniqueID, info.Version
	record.started = fmt.Sprintf("%d:%d", info.UniqueID, info.Version)
	return record, info.Version, nil
}

func signalDarwinToken(token *[8]uint32, signal syscall.Signal) error {
	_, _, errno := syscall.Syscall6(syscall.SYS_PROC_INFO, procSignalAuditToken, 0, uintptr(signal), 0, uintptr(unsafe.Pointer(token)), unsafe.Sizeof(*token))
	if errno != 0 {
		return errno
	}
	return nil
}

func observeProcess(pid int) (processObservation, error) {
	record, _, err := readDarwinProcess(pid)
	if err != nil {
		return processObservation{}, err
	}
	return processObservation{record: record, read: func() (processRecord, error) {
		current, _, err := readDarwinProcess(pid)
		return current, err
	}, close: func() error { return nil }}, nil
}

func openProcessHandle(observed processRecord) (processHandle, error) {
	// A negative PID is never a target of the audit-token API. ESRCH proves
	// this kernel recognizes the primitive without signalling any process.
	probe := [8]uint32{5: ^uint32(0)}
	if err := signalDarwinToken(&probe, syscall.SIGTERM); !errors.Is(err, syscall.ESRCH) {
		return nil, fmt.Errorf("safe descendant teardown requires native audit-token signalling: probe returned %v", err)
	}
	queue, err := syscall.Kqueue()
	if err != nil {
		return nil, err
	}
	syscall.CloseOnExec(queue)
	change := syscall.Kevent_t{Ident: uint64(observed.PID), Filter: syscall.EVFILT_PROC,
		Flags: uint16(syscall.EV_ADD | syscall.EV_ONESHOT), Fflags: uint32(syscall.NOTE_EXIT)}
	// No event buffer means registration completes without waiting.
	if _, err := syscall.Kevent(queue, []syscall.Kevent_t{change}, nil, nil); err != nil {
		_ = syscall.Close(queue)
		return nil, err
	}
	return &darwinProcessHandle{pid: observed.PID, started: observed.started,
		token: [8]uint32{5: uint32(observed.PID), 7: observed.version}, kqueue: queue}, nil
}

func (handle *darwinProcessHandle) signal(signal syscall.Signal) error {
	err := signalDarwinToken(&handle.token, signal)
	if errors.Is(err, syscall.ESRCH) {
		// An exec changes pidversion without ending the process. Refuse
		// visibly instead of waiting forever on the still-live registration.
		current, _, readErr := readDarwinProcess(handle.pid)
		if readErr == nil && current.started != handle.started {
			return fmt.Errorf("recorded process %d changed identity; descendant teardown is incomplete", handle.pid)
		}
		if readErr != nil && !errors.Is(readErr, syscall.ESRCH) {
			return readErr
		}
	}
	return err
}

func (handle *darwinProcessHandle) close() error { return syscall.Close(handle.kqueue) }

func (handle *darwinProcessHandle) wait(ctx context.Context) error {
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
	count, err := syscall.Kevent(handle.kqueue, nil, events, timeout)
	if err != nil {
		return err
	}
	if count == 0 {
		return context.DeadlineExceeded
	}
	if events[0].Flags&syscall.EV_ERROR != 0 {
		return syscall.Errno(events[0].Data)
	}
	if events[0].Filter != syscall.EVFILT_PROC || events[0].Fflags&syscall.NOTE_EXIT == 0 {
		return fmt.Errorf("unexpected process exit event: %+v", events[0])
	}
	return nil
}

func readCurrentProcess(pid int) (processRecord, error) {
	record, _, err := readDarwinProcess(pid)
	return record, err
}
