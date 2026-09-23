package main

import (
	"os"
	"strings"
	"testing"
)

func TestDropReportsMissingNativeState(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "claude-code")
	p, _ := f.run.team.Agent(a.ID)
	if err := os.RemoveAll(p.Directory); err != nil {
		t.Fatal(err)
	}
	if err := f.cmd.drop([]string{"worker"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(f.out.String(), "registration removed") || !strings.Contains(f.out.String(), "native state missing; resume session: unknown") {
		t.Fatalf("silent missing-state cleanup: %q", f.out)
	}
	if _, err := f.run.team.ResolveName("worker"); err == nil {
		t.Fatal("stale registration retained")
	}
}
