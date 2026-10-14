//go:build linux

package tmux

import (
	"bytes"
	"fmt"
	"os"
	"strconv"
)

// ProcessOwner reads pid's parent and command name from /proc/PID/stat and
// its real and effective user IDs from /proc/PID/status.
func ProcessOwner(pid int) (Owner, error) {
	stat, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return Owner{}, err
	}
	status, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return Owner{}, err
	}
	return parseProcOwner(pid, stat, status)
}

// parseProcOwner takes the command name from between the first '(' and the
// last ')' of stat, since the name itself may hold either, and takes the user
// IDs from status's one Uid line (real, effective, saved, filesystem).
func parseProcOwner(pid int, stat, status []byte) (Owner, error) {
	record, err := parseProcStat(string(stat))
	if err != nil {
		return Owner{}, err
	}
	if record.PID != pid {
		return Owner{}, fmt.Errorf("read process %d owner: proc stat names PID %d", pid, record.PID)
	}
	owner := Owner{PID: pid, ParentPID: record.ParentPID, Command: string(stat[bytes.IndexByte(stat, '(')+1 : bytes.LastIndexByte(stat, ')')])}
	found := false
	for _, line := range bytes.Split(status, []byte("\n")) {
		value, ok := bytes.CutPrefix(line, []byte("Uid:"))
		if !ok {
			continue
		}
		if found {
			return Owner{}, fmt.Errorf("read process %d owner: proc status has more than one Uid line", pid)
		}
		found = true
		fields := bytes.Fields(value)
		if len(fields) != 4 {
			return Owner{}, fmt.Errorf("read process %d owner: proc status Uid line has %d fields, not 4", pid, len(fields))
		}
		realID, err := strconv.ParseUint(string(fields[0]), 10, 32)
		if err != nil {
			return Owner{}, fmt.Errorf("read process %d owner: real user ID: %w", pid, err)
		}
		effective, err := strconv.ParseUint(string(fields[1]), 10, 32)
		if err != nil {
			return Owner{}, fmt.Errorf("read process %d owner: effective user ID: %w", pid, err)
		}
		owner.RealUID, owner.UID = uint32(realID), uint32(effective)
	}
	if !found {
		return Owner{}, fmt.Errorf("read process %d owner: proc status has no Uid line", pid)
	}
	return owner, nil
}
