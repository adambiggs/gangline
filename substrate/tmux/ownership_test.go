package tmux

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/adambiggs/gangline/substrate"
)

func TestCurrentOwnedIdentitiesSkipsExitedProcess(t *testing.T) {
	owned := []processIdentity{{pid: 100, started: "root"}, {pid: 200, started: "child"}}
	identities, err := currentOwnedIdentities(owned, "boot", func(pid int) (processRecord, error) {
		if pid == 200 {
			return processRecord{}, &os.PathError{Op: "open", Path: "/proc/200/stat", Err: os.ErrNotExist}
		}
		return processRecord{Process: substrate.Process{PID: 100}, started: "root"}, nil
	})
	if err != nil || len(identities) != 1 || identities[0].PID != 100 {
		t.Fatalf("vanished process stopped teardown preparation: identities=%v err=%v", identities, err)
	}
	_, err = currentOwnedIdentities(owned, "boot", func(int) (processRecord, error) {
		return processRecord{}, os.ErrPermission
	})
	if !errors.Is(err, os.ErrPermission) {
		t.Fatalf("observation error was hidden: %v", err)
	}
}

func TestRecordedTeardownDoesNotCrossBoots(t *testing.T) {
	r, err := readCurrentProcess(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	// A matching PID and start record from another boot must never acquire a handle.
	owned, err := AcquireRecorded([]Identity{{PID: r.PID, Started: r.started, Version: r.version, UniqueID: r.uniqueID, BootID: "another-boot"}})
	if err != nil {
		t.Fatal(err)
	}
	defer owned.Close()
	if len(owned.processes) != 0 {
		t.Fatal("acquired a process from a different boot")
	}
	if err := owned.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
}
