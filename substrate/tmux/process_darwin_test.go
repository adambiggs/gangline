//go:build darwin

package tmux

import (
	"os"
	"testing"
	"unsafe"
)

func TestDarwinNativeProcessIdentity(t *testing.T) {
	var info darwinProcessInfo
	if unsafe.Sizeof(info) != 56 || unsafe.Offsetof(info.UniqueID) != 16 || unsafe.Offsetof(info.Version) != 32 {
		t.Fatalf("proc_info ABI layout changed: size=%d uniqueid=%d version=%d", unsafe.Sizeof(info), unsafe.Offsetof(info.UniqueID), unsafe.Offsetof(info.Version))
	}
	record, _, err := readDarwinProcess(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	if record.PID != os.Getpid() || record.parentUniqueID == 0 || record.started == "" {
		t.Fatalf("native identity = %+v", record)
	}
	observation, err := observeProcess(record.PID)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = observation.close() })
	identity, err := pinObservedProcess(observation.record, observation.read, openProcessHandle)
	if err != nil {
		t.Fatal(err)
	}
	if err := closeOwnedProcesses([]processIdentity{identity}); err != nil {
		t.Fatal(err)
	}
}
