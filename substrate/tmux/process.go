package tmux

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"syscall"

	"github.com/adambiggs/gangline/substrate"
)

type processRecord struct {
	substrate.Process
	foregroundGroup int
	started         string
}

type processIdentity struct {
	pid     int
	started string
}

// ForegroundProcesses returns the descendants of tmux's pane process that
// belong to the terminal's foreground process group. A background child is
// deliberately excluded even when it descends from the harness.
func (backend *Backend) ForegroundProcesses(ctx context.Context, pane substrate.PaneID) ([]substrate.Process, error) {
	if err := validPaneID(pane); err != nil {
		return nil, err
	}
	root, err := backend.paneProcess(ctx, pane)
	if err != nil {
		return nil, err
	}
	records, err := readProcessTable(ctx)
	if err != nil {
		return nil, err
	}
	rootRecord, ok := records[root]
	if !ok {
		return nil, fmt.Errorf("read process tree: pane process %d was not present", root)
	}
	if rootRecord.foregroundGroup <= 0 {
		return nil, fmt.Errorf("read process tree: pane process %d has no foreground process group", root)
	}

	result := make([]substrate.Process, 0, 1)
	for pid, record := range records {
		if record.GroupID == rootRecord.foregroundGroup && descendsFrom(pid, root, records) {
			result = append(result, record.Process)
		}
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("read process tree: foreground process group %d has no pane descendants", rootRecord.foregroundGroup)
	}
	sort.Slice(result, func(left, right int) bool { return result[left].PID < result[right].PID })
	return result, nil
}

func (backend *Backend) paneProcess(ctx context.Context, pane substrate.PaneID) (int, error) {
	output, err := backend.run(ctx, "display-message", "-p", "-t", string(pane), "#{pane_pid}")
	if err != nil {
		return 0, tmuxError("read pane process", err, output)
	}
	root, err := strconv.Atoi(strings.TrimSpace(output))
	if err != nil || root <= 0 {
		return 0, fmt.Errorf("read pane process: tmux returned %q", strings.TrimSpace(output))
	}
	return root, nil
}

func readProcessTable(ctx context.Context) (map[int]processRecord, error) {
	command := exec.CommandContext(ctx, "ps", "-axo", "pid=,ppid=,pgid=,tpgid=,lstart=,comm=")
	command.Env = append(os.Environ(), "LC_ALL=C")
	data, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("read process tree: %w", err)
	}
	return parseProcessTable(string(data))
}

func (backend *Backend) ownedProcesses(ctx context.Context, pane substrate.PaneID) ([]processIdentity, error) {
	root, err := backend.paneProcess(ctx, pane)
	if err != nil {
		return nil, err
	}
	records, err := readProcessTable(ctx)
	if err != nil {
		return nil, err
	}
	if _, ok := records[root]; !ok {
		return nil, fmt.Errorf("read process tree: pane process %d was not present", root)
	}
	owned := make([]processIdentity, 0, 1)
	for pid, record := range records {
		if descendsFrom(pid, root, records) {
			owned = append(owned, processIdentity{pid: pid, started: record.started})
		}
	}
	sort.Slice(owned, func(left, right int) bool { return owned[left].pid > owned[right].pid })
	return owned, nil
}

func reapOwnedProcesses(ctx context.Context, owned []processIdentity) error {
	survivors, err := survivingProcesses(ctx, owned)
	if err != nil {
		return err
	}
	if err := signalProcesses(survivors, syscall.SIGTERM); err != nil {
		return err
	}
	survivors, err = survivingProcesses(ctx, owned)
	if err != nil {
		return err
	}
	if err := signalProcesses(survivors, syscall.SIGKILL); err != nil {
		return err
	}
	for _, survivor := range survivors {
		if err := waitProcessExit(ctx, survivor.pid); err != nil {
			return fmt.Errorf("wait for recorded process %d: %w", survivor.pid, err)
		}
	}
	survivors, err = survivingProcesses(ctx, owned)
	if err != nil {
		return err
	}
	if len(survivors) != 0 {
		pids := make([]string, len(survivors))
		for index, survivor := range survivors {
			pids[index] = strconv.Itoa(survivor.pid)
		}
		return fmt.Errorf("reap pane descendants: recorded processes %s remain after termination", strings.Join(pids, ","))
	}
	return nil
}

func survivingProcesses(ctx context.Context, owned []processIdentity) ([]processIdentity, error) {
	records, err := readProcessTable(ctx)
	if err != nil {
		return nil, err
	}
	return matchingProcesses(owned, records), nil
}

func matchingProcesses(owned []processIdentity, records map[int]processRecord) []processIdentity {
	var survivors []processIdentity
	for _, identity := range owned {
		record, ok := records[identity.pid]
		if ok && record.started == identity.started {
			survivors = append(survivors, identity)
		}
	}
	return survivors
}

func signalProcesses(processes []processIdentity, signal syscall.Signal) error {
	for _, identity := range processes {
		err := syscall.Kill(identity.pid, signal)
		if err != nil && !errors.Is(err, syscall.ESRCH) {
			return fmt.Errorf("signal recorded process %d: %w", identity.pid, err)
		}
	}
	return nil
}

func parseProcessTable(output string) (map[int]processRecord, error) {
	records := make(map[int]processRecord)
	for lineNumber, line := range strings.Split(strings.TrimSpace(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 10 {
			return nil, fmt.Errorf("read process tree: line %d has %d fields", lineNumber+1, len(fields))
		}
		values := make([]int, 4)
		for index := range values {
			value, err := strconv.Atoi(fields[index])
			if err != nil {
				return nil, fmt.Errorf("read process tree: line %d field %d: %w", lineNumber+1, index+1, err)
			}
			values[index] = value
		}
		records[values[0]] = processRecord{
			Process: substrate.Process{
				PID: values[0], ParentPID: values[1], GroupID: values[2], Command: strings.Join(fields[9:], " "),
			},
			foregroundGroup: values[3],
			started:         strings.Join(fields[4:9], " "),
		}
	}
	return records, nil
}

func descendsFrom(pid, root int, records map[int]processRecord) bool {
	seen := make(map[int]bool)
	for pid > 0 && !seen[pid] {
		if pid == root {
			return true
		}
		seen[pid] = true
		record, ok := records[pid]
		if !ok {
			return false
		}
		pid = record.ParentPID
	}
	return false
}
