package main

import (
	"strings"
	"testing"
)

func TestHitchReportsMissingHarnessBeforeCreatingPane(t *testing.T) {
	f := newStateFixture(t)
	t.Setenv("PATH", t.TempDir())
	for _, test := range []struct{ collar, executable string }{
		{"codex", "codex"},
		{"claude-code", "claude"},
	} {
		err := f.cmd.hitch([]string{"worker", "-c", test.collar})
		want := test.executable + ": not found in PATH"
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("hitch -c %s error = %v, want %q", test.collar, err, want)
		}
	}
	agents, err := f.run.team.ListAgents()
	if err != nil {
		t.Fatal(err)
	}
	if len(agents) != 0 {
		t.Fatalf("missing harness claimed agents: %+v", agents)
	}
}
