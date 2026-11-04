//go:build darwin

package tmux

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"syscall"
	"unsafe"

	"github.com/adambiggs/gangline/substrate"
)

// XNU proc_info ABI: PROC_INFO_CALL_LISTPIDS, PROC_ALL_PIDS,
// PROC_PIDTBSDINFO, and PROC_PIDT_SHORTBSDINFO. The BSD record includes the tty
// foreground process group. XNU refuses it with EPERM for a process of another
// effective user unless the caller is privileged; the short record carries no
// tty data and is readable for any process.
const (
	procInfoListPIDs     = 1
	procAllPIDs          = 1
	procPIDTBSDInfo      = 3
	procPIDTShortBSDInfo = 13
)

type darwinBSDInfo struct {
	Flags, Status, ExitStatus, PID, ParentPID uint32
	UID, GID, RealUID, RealGID                uint32
	SavedUID, SavedGID, Reserved              uint32
	Command                                   [16]byte
	Name                                      [32]byte
	Files, GroupID, Jobs, TTYDevice           uint32
	ForegroundGroup                           uint32
	Nice                                      int32
	StartSeconds, StartMicroseconds           uint64
}

type darwinShortBSDInfo struct {
	PID, ParentPID, GroupID, Status uint32
	Command                         [16]byte
	Flags                           uint32
	UID, GID, RealUID, RealGID      uint32
	SavedUID, SavedGID, Reserved    uint32
}

func readProcessTable(ctx context.Context, root int, complete bool) (map[int]processRecord, error) {
	pids, err := listDarwinPIDs(ctx)
	if err != nil {
		return nil, err
	}
	read := readDarwinBSDProcess
	if complete {
		// The shell-only preflight needs ancestry and names, not tty or
		// start-time data. Short records remain readable across users.
		read = readDarwinPaneProcess
	}
	return collectProcessRecords(ctx, root, pids, complete, read)
}

func listDarwinPIDs(ctx context.Context) ([]int, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	needed, _, errno := syscall.Syscall6(syscall.SYS_PROC_INFO, procInfoListPIDs, procAllPIDs, 0, 0, 0, 0)
	if errno != 0 {
		return nil, fmt.Errorf("read process tree: enumerate PIDs: %w", errno)
	}
	if needed == 0 {
		return nil, fmt.Errorf("read process tree: native PID list is empty")
	}
	capacity := int(needed/unsafe.Sizeof(int32(0))) + 32
	for attempt := 0; attempt < 3; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		buffer := make([]int32, capacity)
		count, _, errno := syscall.Syscall6(syscall.SYS_PROC_INFO, procInfoListPIDs, procAllPIDs, 0, 0,
			uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer))*unsafe.Sizeof(buffer[0]))
		if errno != 0 {
			return nil, fmt.Errorf("read process tree: enumerate PIDs: %w", errno)
		}
		if count < uintptr(len(buffer))*unsafe.Sizeof(buffer[0]) {
			pids := make([]int, 0, count/unsafe.Sizeof(buffer[0]))
			for _, pid := range buffer[:count/unsafe.Sizeof(buffer[0])] {
				if pid > 0 {
					pids = append(pids, int(pid))
				}
			}
			return pids, nil
		}
		capacity *= 2
	}
	return nil, fmt.Errorf("read process tree: native PID list changed during enumeration")
}

func readDarwinBSDInfo(pid int) (darwinBSDInfo, error) {
	var info darwinBSDInfo
	size := unsafe.Sizeof(info)
	count, _, errno := syscall.Syscall6(syscall.SYS_PROC_INFO, procInfoPIDInfo, uintptr(pid), procPIDTBSDInfo, 0,
		uintptr(unsafe.Pointer(&info)), size)
	if errno != 0 {
		return darwinBSDInfo{}, errno
	}
	if count == 0 {
		return darwinBSDInfo{}, os.ErrNotExist
	}
	if count != size || info.PID != uint32(pid) {
		return darwinBSDInfo{}, fmt.Errorf("native BSD process data unavailable for %d: returned %d bytes and PID %d", pid, count, info.PID)
	}
	return info, nil
}

// command is the kernel's short process name up to its first NUL.
func (info darwinBSDInfo) command() string {
	return darwinCommand(info.Command[:])
}

func darwinCommand(command []byte) string {
	if end := bytes.IndexByte(command, 0); end >= 0 {
		command = command[:end]
	}
	return string(command)
}

func readDarwinBSDProcess(pid int) (processRecord, error) {
	info, err := readDarwinBSDInfo(pid)
	if err != nil {
		return processRecord{}, err
	}
	return processRecord{
		Process:         substrate.Process{PID: pid, ParentPID: int(info.ParentPID), GroupID: int(info.GroupID), Command: info.command()},
		foregroundGroup: int(info.ForegroundGroup),
		started:         fmt.Sprintf("%d.%06d", info.StartSeconds, info.StartMicroseconds),
	}, nil
}

// ProcessOwner reads pid's parent, command name, and effective (pbsi_uid) and
// real (pbsi_ruid) user IDs from one PROC_PIDT_SHORTBSDINFO record, which an
// unprivileged caller can read for a process of any user, root's included.
func ProcessOwner(pid int) (Owner, error) {
	var info darwinShortBSDInfo
	size := unsafe.Sizeof(info)
	count, _, errno := syscall.Syscall6(syscall.SYS_PROC_INFO, procInfoPIDInfo, uintptr(pid), procPIDTShortBSDInfo, 0,
		uintptr(unsafe.Pointer(&info)), size)
	if errno != 0 {
		return Owner{}, errno
	}
	if count == 0 {
		return Owner{}, os.ErrNotExist
	}
	if count != size || info.PID != uint32(pid) {
		return Owner{}, fmt.Errorf("native short BSD process data unavailable for %d: returned %d bytes and PID %d", pid, count, info.PID)
	}
	return Owner{PID: pid, ParentPID: int(info.ParentPID), UID: info.UID, RealUID: info.RealUID, Command: darwinCommand(info.Command[:])}, nil
}

// readDarwinPaneProcess supplies only the fields used by PaneProcesses.
func readDarwinPaneProcess(pid int) (processRecord, error) {
	owner, err := ProcessOwner(pid)
	if err != nil {
		return processRecord{}, err
	}
	return processRecord{Process: substrate.Process{PID: owner.PID, ParentPID: owner.ParentPID, Command: owner.Command}}, nil
}
