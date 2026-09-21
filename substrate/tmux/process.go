package tmux

import (
	"context"
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"

	"github.com/adambiggs/gangline/substrate"
)

type processRecord struct {
	substrate.Process
	foregroundGroup int
}

// ForegroundProcesses returns the descendants of tmux's pane process that
// belong to the terminal's foreground process group. A background child is
// deliberately excluded even when it descends from the harness.
func (backend *Backend) ForegroundProcesses(ctx context.Context, pane substrate.PaneID) ([]substrate.Process, error) {
	if err := validPaneID(pane); err != nil {
		return nil, err
	}
	output, err := backend.run(ctx, "display-message", "-p", "-t", string(pane), "#{pane_pid}")
	if err != nil {
		return nil, tmuxError("read pane process", err, output)
	}
	root, err := strconv.Atoi(strings.TrimSpace(output))
	if err != nil || root <= 0 {
		return nil, fmt.Errorf("read pane process: tmux returned %q", strings.TrimSpace(output))
	}

	command := exec.CommandContext(ctx, "ps", "-axo", "pid=,ppid=,pgid=,tpgid=,comm=")
	data, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("read process tree: %w", err)
	}
	records, err := parseProcessTable(string(data))
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

func parseProcessTable(output string) (map[int]processRecord, error) {
	records := make(map[int]processRecord)
	for lineNumber, line := range strings.Split(strings.TrimSpace(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 5 {
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
				PID: values[0], ParentPID: values[1], GroupID: values[2], Command: strings.Join(fields[4:], " "),
			},
			foregroundGroup: values[3],
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
