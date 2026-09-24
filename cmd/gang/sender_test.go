package main

import (
	"testing"

	"github.com/adambiggs/gangline/core"
)

func TestInterruptReasonCarriesGanglineSender(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "codex")
	p, _ := f.run.team.Agent(a.ID)
	if err := f.cmd.interrupt([]string{"worker", "-m", "stop and report"}); err != nil {
		t.Fatal(err)
	}
	pending, err := p.ListNew()
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 || pending[0].From != (core.Sender{Kind: core.SenderGangline, Name: "interrupt"}) {
		t.Fatalf("interrupt reason: %+v", pending)
	}
}
