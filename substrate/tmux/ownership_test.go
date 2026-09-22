package tmux

import (
	"context"
	"os"
	"testing"
)

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
