package tmux

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
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
