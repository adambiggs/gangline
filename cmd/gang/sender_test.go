package main

import (
	"errors"
	"strings"
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
	wire, err := envelopeText(pending[0])
	if err != nil || len(pending[0].Token) != 16 || !strings.Contains(wire, "#"+pending[0].Token+"]") || strings.Contains(wire, string(pending[0].ID)) {
		t.Fatalf("interrupt token: %+v wire=%q err=%v", pending[0], wire, err)
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

// An interrupt leaves a native approval prompt for the operator: its keys
// would answer the prompt as a refusal.
func TestInterruptRefusesBlockedNativeInput(t *testing.T) {
	for _, collar := range []string{"codex", "claude"} {
		t.Run(collar, func(t *testing.T) {
			f := newStateFixture(t)
			a := f.add(t, "a", "worker", collar)
			f.input.screen = compactInterruptScreens(t, f, collar).blocked
			err := f.cmd.interrupt([]string{"worker", "-m", "stop and report"})
			var refused commandError
			if !errors.As(err, &refused) || refused.status != exitRefused || !strings.Contains(err.Error(), "native input blocked") {
				t.Fatalf("interrupt on an approval prompt: %v", err)
			}
			p, _ := f.run.team.Agent(a.ID)
			requireNothingTyped(t, f, p)
			got, err := p.Read()
			if err != nil {
				t.Fatal(err)
			}
			if !got.InterruptDeadline.IsZero() || got.Activity == core.Interrupting || got.Input != nil {
				t.Fatalf("refused interrupt left state: activity=%s deadline=%v input=%+v", got.Activity, got.InterruptDeadline, got.Input)
			}
		})
	}
}
