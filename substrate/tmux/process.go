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
	foregroundGroup          int
	started                  string
	uniqueID, parentUniqueID uint64
	version                  uint32
}

// processObservation retains the native identity source until all selected
// ancestors have been validated and their signalling handles acquired.
type processObservation struct {
	record processRecord
	read   func() (processRecord, error)
	close  func() error
}

type processIdentity struct {
	pid     int
	started string
	handle  processHandle
}

// A handle binds signalling and exit observation to one native process.
// A numeric PID, even paired with a timestamp, cannot implement this contract.
type processHandle interface {
	signal(syscall.Signal) error
	wait(context.Context) error
	close() error
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
	output, err := backend.run(ctx, "display-message", "-p", "-t", string(pane), "#{pane_pid} #{pane_dead}")
	if err != nil {
		return 0, tmuxError("read pane process", err, output)
	}
	fields := strings.Fields(output)
	if len(fields) != 2 || fields[1] != "0" {
		return 0, fmt.Errorf("read pane process: no live pane process in %q", strings.TrimSpace(output))
	}
	root, err := strconv.Atoi(fields[0])
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

func (backend *Backend) ownedProcesses(ctx context.Context, pane substrate.PaneID) (owned []processIdentity, result error) {
	root, err := backend.paneProcess(ctx, pane)
	if err != nil {
		return nil, err
	}
	// Capture the root before enumerating the rest. In particular, do not
	// relabel a coarse ps snapshot with a replacement root's native identity.
	rootObservation, err := observeProcess(root)
	if err != nil {
		return nil, err
	}
	observations := map[int]processObservation{root: rootObservation}
	defer func() {
		for _, observation := range observations {
			result = errors.Join(result, observation.close())
		}
		if result != nil {
			result = errors.Join(result, closeOwnedProcesses(owned))
			owned = nil
		}
	}()
	enumerated, err := readProcessTable(ctx)
	if err != nil {
		return nil, err
	}
	if err := observeProcessCandidates(root, enumerated, observations, observeProcess); err != nil {
		return nil, err
	}
	owned, err = pinOwnedProcesses(root, nativeAncestry(observations), func(record processRecord) (processIdentity, error) {
		observation := observations[record.PID]
		return pinObservedProcess(observation.record, observation.read, openProcessHandle)
	})
	if err != nil {
		return nil, err
	}
	// tmux must still associate this live pane with the captured root. A
	// dead pane can retain a PID that has already been reused elsewhere.
	currentRoot, err := backend.paneProcess(ctx, pane)
	if err != nil {
		return owned, err
	}
	if currentRoot != root {
		return owned, fmt.Errorf("pane process changed during identity acquisition")
	}
	return owned, nil
}

func observeProcessCandidates(root int, enumerated map[int]processRecord, observations map[int]processObservation, observe func(int) (processObservation, error)) error {
	// ps bounds descriptor use to possible descendants. Membership here
	// authorizes only an observation; retained native ancestry and identity
	// validation below decide which processes can receive signals.
	for pid := range enumerated {
		if pid == root || !descendsFrom(pid, root, enumerated) {
			continue
		}
		observation, err := observe(pid)
		if errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ESRCH) {
			continue
		}
		if err != nil {
			return fmt.Errorf("observe process %d: %w", pid, err)
		}
		observations[pid] = observation
	}
	return nil
}

func nativeAncestry(observations map[int]processObservation) map[int]processRecord {
	records := make(map[int]processRecord, len(observations))
	uniquePIDs := make(map[uint64]int, len(observations))
	for pid, observation := range observations {
		if observation.record.uniqueID != 0 {
			uniquePIDs[observation.record.uniqueID] = pid
		}
	}
	for pid, observation := range observations {
		record := observation.record
		if record.uniqueID != 0 {
			// Darwin provides the parent's native unique ID in the same
			// observation, so reused numeric PIDs cannot join two lineages.
			record.ParentPID = uniquePIDs[record.parentUniqueID]
		}
		records[pid] = record
	}
	return records
}

func pinOwnedProcesses(root int, records map[int]processRecord, open func(processRecord) (processIdentity, error)) ([]processIdentity, error) {
	if _, ok := records[root]; !ok {
		return nil, fmt.Errorf("read process tree: pane process %d was not present", root)
	}
	pids := make([]int, 0, len(records))
	for pid := range records {
		if descendsFrom(pid, root, records) {
			pids = append(pids, pid)
		}
	}
	sort.Sort(sort.Reverse(sort.IntSlice(pids)))
	owned := make([]processIdentity, 0, len(pids))
	for _, pid := range pids {
		identity, err := open(records[pid])
		if err != nil {
			return nil, errors.Join(fmt.Errorf("pin recorded process %d: %w", pid, err), closeOwnedProcesses(owned))
		}
		owned = append(owned, identity)
	}
	return owned, nil
}

func closeOwnedProcesses(owned []processIdentity) error {
	var result error
	for _, identity := range owned {
		if identity.handle != nil {
			if err := identity.handle.close(); err != nil {
				result = errors.Join(result, fmt.Errorf("release recorded process %d: %w", identity.pid, err))
			}
		}
	}
	return result
}

func reapOwnedProcesses(ctx context.Context, owned []processIdentity) error {
	if err := signalProcesses(owned, syscall.SIGTERM); err != nil {
		return err
	}
	if err := signalProcesses(owned, syscall.SIGKILL); err != nil {
		return err
	}
	for _, identity := range owned {
		if err := identity.handle.wait(ctx); err != nil {
			return fmt.Errorf("wait for recorded process %d: %w", identity.pid, err)
		}
	}
	return nil
}

// pinObservedProcess reads the native identity on both sides of handle
// acquisition. Linux supplies reads from one open procfs file, so a recycled
// PID cannot make either read refer to a replacement, even within one tick.
func pinObservedProcess(expected processRecord, read func() (processRecord, error), open func(processRecord) (processHandle, error)) (processIdentity, error) {
	before, err := read()
	if err != nil {
		return processIdentity{}, err
	}
	if before.PID != expected.PID || before.ParentPID != expected.ParentPID || before.started == "" || before.started != expected.started || before.parentUniqueID != expected.parentUniqueID {
		return processIdentity{}, fmt.Errorf("process %d changed before identity acquisition", expected.PID)
	}
	handle, err := open(before)
	if err != nil {
		return processIdentity{}, err
	}
	after, err := read()
	if err == nil && (after.PID != before.PID || after.ParentPID != before.ParentPID || after.started != before.started || after.parentUniqueID != before.parentUniqueID) {
		err = fmt.Errorf("process %d changed during identity acquisition", expected.PID)
	}
	if err != nil {
		return processIdentity{}, errors.Join(err, handle.close())
	}
	return processIdentity{pid: before.PID, started: before.started, handle: handle}, nil
}

func signalProcesses(processes []processIdentity, signal syscall.Signal) error {
	for _, identity := range processes {
		if identity.handle == nil {
			return fmt.Errorf("signal recorded process %d: no pinned process identity", identity.pid)
		}
		err := identity.handle.signal(signal)
		if err != nil && !errors.Is(err, syscall.ESRCH) && !errors.Is(err, os.ErrProcessDone) {
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
