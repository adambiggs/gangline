//go:build darwin

package tmux

import (
	"context"
	"fmt"
	"os"
	"syscall"
	"unsafe"

	"github.com/adambiggs/gangline/substrate"
)

// XNU proc_info ABI: PROC_INFO_CALL_LISTPIDS, PROC_ALL_PIDS, and
// PROC_PIDTBSDINFO. The BSD record includes the tty foreground process group.
const (
	procInfoListPIDs = 1
	procAllPIDs      = 1
	procPIDTBSDInfo  = 3
	kernProcArgs2    = 49
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

func readProcessTable(ctx context.Context, root int) (map[int]processRecord, error) {
	pids, err := listDarwinPIDs(ctx)
	if err != nil {
		return nil, err
	}
	return collectProcessRecords(ctx, root, pids, readDarwinBSDProcess)
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

func readDarwinBSDProcess(pid int) (processRecord, error) {
	var info darwinBSDInfo
	size := unsafe.Sizeof(info)
	count, _, errno := syscall.Syscall6(syscall.SYS_PROC_INFO, procInfoPIDInfo, uintptr(pid), procPIDTBSDInfo, 0,
		uintptr(unsafe.Pointer(&info)), size)
	if errno != 0 {
		return processRecord{}, errno
	}
	if count == 0 {
		return processRecord{}, os.ErrNotExist
	}
	if count != size || info.PID != uint32(pid) {
		return processRecord{}, fmt.Errorf("native BSD process data unavailable for %d: returned %d bytes and PID %d", pid, count, info.PID)
	}
	command := info.Command[:]
	if end := indexZero(command); end >= 0 {
		command = command[:end]
	}
	return processRecord{
		Process:         substrate.Process{PID: pid, ParentPID: int(info.ParentPID), GroupID: int(info.GroupID), Command: string(command)},
		foregroundGroup: int(info.ForegroundGroup),
		started:         fmt.Sprintf("%d.%06d", info.StartSeconds, info.StartMicroseconds),
	}, nil
}

func foregroundCommand(process substrate.Process) string {
	command, err := readDarwinArgv0(process.PID)
	if err != nil {
		return process.Command
	}
	return command
}

func readDarwinArgv0(pid int) (string, error) {
	maxArgs, err := syscall.SysctlUint32("kern.argmax")
	if err != nil {
		return "", err
	}
	if maxArgs < 8 {
		return "", fmt.Errorf("native process arguments have invalid capacity %d", maxArgs)
	}
	buffer := make([]byte, maxArgs)
	mib := [3]int32{1, kernProcArgs2, int32(pid)} // CTL_KERN, KERN_PROCARGS2, pid.
	length := uintptr(len(buffer))
	_, _, errno := syscall.Syscall6(syscall.SYS___SYSCTL, uintptr(unsafe.Pointer(&mib[0])), uintptr(len(mib)),
		uintptr(unsafe.Pointer(&buffer[0])), uintptr(unsafe.Pointer(&length)), 0, 0)
	if errno != 0 {
		return "", errno
	}
	if length > uintptr(len(buffer)) {
		return "", fmt.Errorf("native process arguments exceeded allocated capacity")
	}
	return parseDarwinArgv0(buffer[:length])
}
