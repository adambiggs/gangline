package tmux

import (
	"context"
	"errors"
	"os"
	"strings"
	"syscall"
	"testing"

	"github.com/adambiggs/gangline/substrate"
)

func TestSignalRecordedProcessRequiresPinnedIdentity(t *testing.T) {
	// Signal zero cannot terminate anything. A PID and a sampled timestamp
	// alone must nevertheless never authorize the production signal path.
	err := signalProcesses([]processIdentity{{pid: os.Getpid(), started: "unverified"}}, syscall.Signal(0))
	if err == nil {
		t.Fatal("numeric PID reached the signal path without a pinned identity")
	}
}

type fakeProcessHandle struct {
	signals       []syscall.Signal
	waits, closes int
	closeErr      error
}

func (handle *fakeProcessHandle) signal(signal syscall.Signal) error {
	handle.signals = append(handle.signals, signal)
	return nil
}
func (handle *fakeProcessHandle) wait(context.Context) error { handle.waits++; return nil }
func (handle *fakeProcessHandle) close() error               { handle.closes++; return handle.closeErr }

func TestPinObservedProcessRejectsReplacementDuringAcquisition(t *testing.T) {
	before := processRecord{Process: substrate.Process{PID: 200, ParentPID: 100}, started: "101"}
	after := before
	after.started = "102" // Native ticks can differ within the same ps lstart second.
	handle := &fakeProcessHandle{}
	current := before
	_, err := pinObservedProcess(before, func() (processRecord, error) { return current, nil },
		func(processRecord) (processHandle, error) { current = after; return handle, nil })
	if err == nil || !strings.Contains(err.Error(), "changed during identity acquisition") {
		t.Fatalf("replacement refusal = %v", err)
	}
	if handle.closes != 1 || len(handle.signals) != 0 {
		t.Fatalf("rejected handle: closes=%d signals=%v", handle.closes, handle.signals)
	}
}

func TestPinObservedProcessKeepsHandleWhenValidationDisappears(t *testing.T) {
	record := processRecord{Process: substrate.Process{PID: 200, ParentPID: 100}, started: "101"}
	handle := &fakeProcessHandle{}
	acquired := false
	identity, err := pinObservedProcess(record, func() (processRecord, error) {
		if acquired {
			return processRecord{}, syscall.ESRCH
		}
		return record, nil
	}, func(processRecord) (processHandle, error) { acquired = true; return handle, nil })
	if err != nil || identity.handle != handle || handle.closes != 0 {
		t.Fatalf("vanished process lost its pinned handle: identity=%+v err=%v closes=%d", identity, err, handle.closes)
	}
	if err := identity.handle.close(); err != nil {
		t.Fatal(err)
	}
}

func TestPinnedSignalAndWaitDoNotFollowReplacementPID(t *testing.T) {
	record := processRecord{Process: substrate.Process{PID: os.Getpid(), ParentPID: os.Getppid()}, started: "101"}
	original, replacement := &fakeProcessHandle{}, &fakeProcessHandle{}
	current := original
	identity, err := pinObservedProcess(record, func() (processRecord, error) { return record, nil },
		func(processRecord) (processHandle, error) { return current, nil })
	if err != nil {
		t.Fatal(err)
	}
	current = replacement
	// Zero keeps this regression harmless even if numeric PID signalling returns.
	if err := signalProcesses([]processIdentity{identity}, syscall.Signal(0)); err != nil {
		t.Fatal(err)
	}
	if err := identity.handle.wait(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := closeOwnedProcesses([]processIdentity{identity}); err != nil {
		t.Fatal(err)
	}
	if len(original.signals) != 1 || original.waits != 1 || original.closes != 1 || len(current.signals) != 0 || current.waits != 0 {
		t.Fatalf("signals/wait followed replacement: original=%+v replacement=%+v", original, current)
	}
}

func TestPinOwnedProcessesReleasesEarlierHandlesOnFailure(t *testing.T) {
	records := map[int]processRecord{
		100: {Process: substrate.Process{PID: 100, ParentPID: 1}},
		200: {Process: substrate.Process{PID: 200, ParentPID: 100}},
	}
	handle := &fakeProcessHandle{}
	_, err := pinOwnedProcesses(100, records, func(record processRecord) (processIdentity, error) {
		if record.PID == 100 {
			return processIdentity{}, syscall.EPERM
		}
		return processIdentity{pid: record.PID, handle: handle}, nil
	})
	if !errors.Is(err, syscall.EPERM) || handle.closes != 1 {
		t.Fatalf("partial acquisition cleanup: err=%v closes=%d", err, handle.closes)
	}
}

func TestPinOwnedProcessesSkipsVanishedChild(t *testing.T) {
	root := processRecord{Process: substrate.Process{PID: 100}, started: "root"}
	child := processRecord{Process: substrate.Process{PID: 200, ParentPID: 100}, started: "child"}
	records := map[int]processRecord{100: root, 200: child}
	owned, err := pinOwnedProcesses(100, records, func(record processRecord) (processIdentity, error) {
		if record.PID == 200 {
			return processIdentity{}, &os.PathError{Op: "open", Path: "/proc/200/stat", Err: os.ErrNotExist}
		}
		return processIdentity{pid: record.PID, handle: &fakeProcessHandle{}}, nil
	})
	if err != nil || len(owned) != 1 || owned[0].pid != 100 {
		t.Fatalf("vanished child stopped acquisition: owned=%v err=%v", owned, err)
	}
	if err := closeOwnedProcesses(owned); err != nil {
		t.Fatal(err)
	}
}

func TestPinOwnedProcessesReleasesChildrenWhenRootVanishes(t *testing.T) {
	records := map[int]processRecord{
		100: {Process: substrate.Process{PID: 100}},
		200: {Process: substrate.Process{PID: 200, ParentPID: 100}},
	}
	childHandle := &fakeProcessHandle{}
	owned, err := pinOwnedProcesses(100, records, func(record processRecord) (processIdentity, error) {
		if record.PID == 100 {
			return processIdentity{}, os.ErrNotExist
		}
		return processIdentity{pid: record.PID, handle: childHandle}, nil
	})
	if err != nil || len(owned) != 0 || childHandle.closes != 1 {
		t.Fatalf("vanished root: owned=%v err=%v child closes=%d", owned, err, childHandle.closes)
	}
}

func TestPinObservedProcessRejectsReplacementBeforeFirstRead(t *testing.T) {
	expected := processRecord{Process: substrate.Process{PID: 200, ParentPID: 100}, started: "101"}
	replacement := expected
	replacement.started = "102"
	opened := false
	_, err := pinObservedProcess(expected, func() (processRecord, error) { return replacement, nil },
		func(processRecord) (processHandle, error) { opened = true; return &fakeProcessHandle{}, nil })
	if err == nil || opened {
		t.Fatalf("replacement before first read was admitted: err=%v opened=%v", err, opened)
	}
}

func TestNativeAncestryRejectsReusedParentPID(t *testing.T) {
	observations := map[int]processObservation{
		100: {record: processRecord{Process: substrate.Process{PID: 100}, uniqueID: 20, started: "20:1"}},
		200: {record: processRecord{Process: substrate.Process{PID: 200, ParentPID: 100}, uniqueID: 30, parentUniqueID: 10, started: "30:1"}},
	}
	if descendsFrom(200, 100, nativeAncestry(observations)) {
		t.Fatal("child of old parent identity inherited replacement parent's pane ownership")
	}
	observations[200] = processObservation{record: processRecord{Process: substrate.Process{PID: 200}, uniqueID: 30, parentUniqueID: 20, started: "30:1"}}
	if !descendsFrom(200, 100, nativeAncestry(observations)) {
		t.Fatal("matching native parent identity lost pane ancestry")
	}
}

func TestPinOwnedProcessesRejectsChangedAncestor(t *testing.T) {
	root := processRecord{Process: substrate.Process{PID: 100, ParentPID: 1}, started: "101"}
	child := processRecord{Process: substrate.Process{PID: 200, ParentPID: 100}, started: "201"}
	records := map[int]processRecord{100: root, 200: child}
	replacement := root
	replacement.started = "102"
	childHandle := &fakeProcessHandle{}
	_, err := pinOwnedProcesses(100, records, func(expected processRecord) (processIdentity, error) {
		current := expected
		if current.PID == 100 {
			current = replacement
		}
		return pinObservedProcess(expected, func() (processRecord, error) { return current, nil }, func(processRecord) (processHandle, error) { return childHandle, nil })
	})
	if err == nil || childHandle.closes != 1 || len(childHandle.signals) != 0 {
		t.Fatalf("changed root authorized previously pinned child: err=%v handle=%+v", err, childHandle)
	}
}

func TestObserveProcessCandidatesBoundsFilesAndUsesNativeAncestry(t *testing.T) {
	root := processRecord{Process: substrate.Process{PID: 100, ParentPID: 1}, started: "101"}
	enumerated := map[int]processRecord{
		100: root,
		200: {Process: substrate.Process{PID: 200, ParentPID: 100}},
		300: {Process: substrate.Process{PID: 300, ParentPID: 200}},
		900: {Process: substrate.Process{PID: 900, ParentPID: 1}},
		901: {Process: substrate.Process{PID: 901, ParentPID: 900}},
	}
	observations := map[int]processObservation{100: {record: root}}
	opened := make(map[int]bool)
	err := observeProcessCandidates(100, enumerated, observations, func(pid int) (processObservation, error) {
		opened[pid] = true
		if pid != 200 && pid != 300 {
			return processObservation{}, syscall.EMFILE
		}
		record := enumerated[pid]
		record.started = "native"
		if pid == 200 {
			// The ps candidate moved under an unrelated native ancestor.
			// Its child must not inherit pane ownership from the old list.
			record.ParentPID = 900
		}
		return processObservation{record: record}, nil
	})
	if err != nil || len(opened) != 2 || !opened[200] || !opened[300] {
		t.Fatalf("native files were not bounded to pane candidates: opened=%v err=%v", opened, err)
	}
	var pinned []int
	owned, err := pinOwnedProcesses(100, nativeAncestry(observations), func(record processRecord) (processIdentity, error) {
		pinned = append(pinned, record.PID)
		return processIdentity{pid: record.PID, handle: &fakeProcessHandle{}}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := closeOwnedProcesses(owned); err != nil {
		t.Fatal(err)
	}
	if len(pinned) != 1 || pinned[0] != 100 {
		t.Fatalf("coarse ps ancestry authorized changed native branch: pinned=%v", pinned)
	}
}
