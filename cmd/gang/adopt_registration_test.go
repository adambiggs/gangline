package main

import (
	"strings"
	"testing"
)

func TestAdoptRefusesAlreadyRegisteredPane(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "codex")
	f.env["TMUX_PANE"] = a.Pane
	if err := f.cmd.adopt([]string{"another", "-c", "codex"}); err == nil || !strings.Contains(err.Error(), "already registered") {
		t.Fatalf("adopt=%v", err)
	}
	agents, err := f.run.team.ListAgents()
	if err != nil || len(agents) != 1 {
		t.Fatalf("agents=%+v err=%v", agents, err)
	}
}
