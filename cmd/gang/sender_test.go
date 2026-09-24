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

func TestCompactionStoresResumeProvenance(t *testing.T) {
	for _, custom := range []bool{false, true} {
		f := newStateFixture(t)
		a := f.add(t, "a", "worker", "codex")
		// A busy surface keeps the compaction queued for direct state inspection.
		f.input.screen = screenWithText("esc to interrupt", "› ")
		args := []string{"worker"}
		want := core.Sender{Kind: core.SenderGangline, Name: "compact"}
		if custom {
			args = append(args, "--resume", "resume saved work")
			want.Kind = core.SenderSelfDeclared
		}
		if err := f.cmd.compact(args); err != nil {
			t.Fatal(err)
		}
		p, _ := f.run.team.Agent(a.ID)
		got, err := p.Read()
		if err != nil {
			t.Fatal(err)
		}
		if got.Compaction == nil || got.Compaction.ResumeFrom != want {
			t.Fatalf("custom=%v compaction=%+v want=%+v", custom, got.Compaction, want)
		}
	}
}
