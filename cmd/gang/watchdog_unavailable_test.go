package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type unavailableWatchdog struct{ *fakeWatchdog }

func (unavailableWatchdog) Available() error {
	return errors.New("Failed to connect to bus: No data available")
}

func TestUnavailableUserBusDoesNotFailCompletedDrop(t *testing.T) {
	f, s := watchdogFixture(t)
	if _, err := f.run.updateWatchdog("", false, false); err != nil {
		t.Fatal(err)
	}
	f.cmd.newScheduler = func() watchdogScheduler { return unavailableWatchdog{s} }
	if err := f.cmd.drop([]string{"worker"}); err != nil {
		t.Fatalf("completed drop failed: %v", err)
	}
	if s.stops != 0 {
		t.Fatal("attempted inaccessible timer cancellation")
	}
	if !strings.Contains(f.errOut.String(), "cancellation deferred") {
		t.Fatalf("skip was silent: %s", f.errOut)
	}
	log, err := os.ReadFile(filepath.Join(f.run.team.Directory, "log.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(log), `"type":"watchdog_unavailable"`) || !strings.Contains(string(log), "No data available") {
		t.Fatalf("missing bus evidence: %s", log)
	}
	if _, err := f.run.team.ResolveName("worker"); err == nil {
		t.Fatal("registration survived drop")
	}
}
