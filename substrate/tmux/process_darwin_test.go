//go:build acceptance && darwin

package tmux

import (
	"context"
	"os"
	"syscall"
	"testing"
	"unsafe"
)

func TestDarwinBootIdentityMatchesBootSessionUUID(t *testing.T) {
	want, err := syscall.Sysctl("kern.bootsessionuuid")
	if err != nil || want == "" {
		t.Fatalf("native boot session UUID = %q, %v", want, err)
	}
	got, err := bootIdentity()
	if err != nil || got != want {
		t.Fatalf("native boot identity = %q, %v; want boot session UUID %q", got, err, want)
	}
}

func TestDarwinNativeProcessTable(t *testing.T) {
	var info darwinBSDInfo
	if unsafe.Sizeof(info) != 136 || unsafe.Offsetof(info.GroupID) != 100 || unsafe.Offsetof(info.ForegroundGroup) != 112 || unsafe.Offsetof(info.StartSeconds) != 120 {
		t.Fatalf("proc_bsdinfo ABI layout changed: size=%d pgid=%d tpgid=%d start=%d", unsafe.Sizeof(info), unsafe.Offsetof(info.GroupID), unsafe.Offsetof(info.ForegroundGroup), unsafe.Offsetof(info.StartSeconds))
	}
	record, err := readDarwinBSDProcess(os.Getpid())
	if err != nil || record.PID != os.Getpid() || record.ParentPID != os.Getppid() || record.GroupID <= 0 || record.started == "" {
		t.Fatalf("native BSD record = %+v, %v", record, err)
	}
	records, err := readProcessTable(context.Background(), os.Getpid(), false)
	if err != nil || records[os.Getpid()].PID != os.Getpid() {
		t.Fatalf("native process table = %+v, %v", records[os.Getpid()], err)
	}
}

func TestDarwinCallerAncestry(t *testing.T) {
	caller := os.Getpid()
	parent := os.Getppid()
	record, err := readCallerProcess(caller)
	if err != nil || record.ParentPID != parent {
		t.Fatalf("caller parent = %d, want %d: %v", record.ParentPID, parent, err)
	}
	if err := verifyCallerAncestry(caller, parent, readCallerProcess); err != nil {
		t.Fatal(err)
	}
}

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
