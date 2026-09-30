package tmux

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/adambiggs/gangline/substrate"
)

func TestProcessVisibilityReadErrors(t *testing.T) {
	unexpected := errors.New("malformed process record")
	for _, err := range []error{nil, os.ErrNotExist, syscall.ESRCH, os.ErrPermission, unexpected} {
		visible, got := processReadVisibility(err)
		if visible != (err == nil) || (err == unexpected && !errors.Is(got, unexpected)) || (err != unexpected && got != nil) {
			t.Fatalf("read error %v: visibility = %v, %v", err, visible, got)
		}
	}
}

func TestRecordedProcessNamespace(t *testing.T) {
	namespace, err := nativeProcessNamespace()
	if err != nil {
		t.Fatal(err)
	}
	boot, err := bootIdentity()
	if err != nil {
		t.Fatal(err)
	}
	for _, saved := range []string{"", "another-namespace", namespace} {
		id := Identity{PID: os.Getpid(), BootID: boot, Namespace: saved}
		if got := CanReadIdentity(id); got != (saved == namespace) {
			t.Fatalf("namespace %q readable = %v", saved, got)
		}
		if saved != namespace {
			if owned, err := AcquireRecorded([]Identity{id}); err == nil {
				owned.Close()
				t.Fatalf("unverified namespace %q acquired", saved)
			}
		}
	}
}

func TestAcquireTreeRetainsRecordedRootAfterPaneLoss(t *testing.T) {
	r, err := readCurrentProcess(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	boot, err := bootIdentity()
	if err != nil {
		t.Fatal(err)
	}
	namespace, err := nativeProcessNamespace()
	if err != nil {
		t.Fatal(err)
	}
	id := Identity{PID: r.PID, Started: r.started, Version: r.version, UniqueID: r.uniqueID, BootID: boot, Namespace: namespace}
	binary := filepath.Join(t.TempDir(), "absent-tmux")
	if err := os.WriteFile(binary, []byte("#!/bin/sh\necho 'no server running on /tmp/absent.sock' >&2\nexit 1\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	b, _ := New(Config{Binary: binary, Session: "absent"})
	owned, err := b.AcquireTree(context.Background(), "%1", id)
	if err != nil {
		t.Fatal(err)
	}
	defer owned.Close()
	if ids := owned.Identities(); len(ids) != 1 || ids[0] != id {
		t.Fatalf("retained process identities = %v, want %v", ids, id)
	}
}

func TestCallerAncestryJoinsDarwinParentAndUniqueIdentities(t *testing.T) {
	native := map[int]processRecord{
		3: {Process: substrate.Process{PID: 3}, uniqueID: 30, parentUniqueID: 20, version: 1},
		2: {Process: substrate.Process{PID: 2}, uniqueID: 20, parentUniqueID: 10, version: 1},
		1: {Process: substrate.Process{PID: 1}, uniqueID: 10, version: 1},
	}
	parents := map[int]int{3: 2, 2: 1}
	readNative := func(pid int) (processRecord, error) { return native[pid], nil }
	readBSD := func(pid int) (processRecord, error) {
		return processRecord{Process: substrate.Process{PID: pid, ParentPID: parents[pid]}}, nil
	}
	if err := verifyCallerAncestry(3, 1, readNative); err == nil {
		t.Fatal("native identity record unexpectedly contained parent links")
	}
	readJoined := func(pid int) (processRecord, error) {
		return callerProcessWithParent(pid, readNative, readBSD)
	}
	if err := verifyCallerAncestry(3, 1, readJoined); err != nil {
		t.Fatal(err)
	}
	native[2] = processRecord{Process: substrate.Process{PID: 2}, uniqueID: 99, parentUniqueID: 10, version: 1}
	if err := verifyCallerAncestry(3, 1, readJoined); err == nil {
		t.Fatal("reused parent PID joined the caller lineage")
	}
	native[2] = processRecord{Process: substrate.Process{PID: 2}, uniqueID: 20, parentUniqueID: 10, version: 1}
	reads := 0
	changedNative := func(pid int) (processRecord, error) {
		reads++
		record := native[pid]
		if reads == 2 {
			record.version++
		}
		return record, nil
	}
	if _, err := callerProcessWithParent(3, changedNative, readBSD); err == nil {
		t.Fatal("process changed between native and parent observations")
	}
}
