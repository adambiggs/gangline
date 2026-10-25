package main

import (
	"strings"
	"testing"

	"github.com/adambiggs/gangline/core"
)

func TestCompactInterruptCancelsQueuedRequest(t *testing.T) {
	for _, collar := range []string{"codex", "claude"} {
		t.Run(collar, func(t *testing.T) {
			f := newStateFixture(t)
			a := f.add(t, "a", "worker", collar)
			screens := compactInterruptScreens(t, f, collar)
			f.input.screen = screens.busy
			if err := f.cmd.compact([]string{"worker", "--note", "continue saved work"}); err != nil {
				t.Fatal(err)
			}
			p, _ := f.run.team.Agent(a.ID)
			before, err := p.Read()
			if err != nil || before.Compaction == nil || before.Compaction.Status != "queued" {
				t.Fatalf("queue: %+v, %v", before.Compaction, err)
			}
			if err := f.cmd.compact([]string{"worker", "--interrupt"}); err != nil {
				t.Fatal(err)
			}
			after, err := p.Read()
			if err != nil || after.Compaction != nil || after.Activity != before.Activity {
				t.Fatalf("cancel: %+v, %v", after, err)
			}
			if !strings.Contains(f.out.String(), before.Compaction.ID+"\tqueued compaction cancelled") {
				t.Fatalf("missing cancellation result: %s", f.out)
			}
			if len(f.input.keys) != 0 || f.input.submits != 0 || f.input.pasted != "" {
				t.Fatalf("cancellation typed native input: %+v", f.input)
			}
			f.input.screen = screens.idle
			if err := f.run.tickAgent(a.ID, hookNotice{}, false); err != nil {
				t.Fatal(err)
			}
			if f.input.submits != 0 || f.input.pasted != "" {
				t.Fatal("cancelled compaction ran at idle")
			}
			pending, err := p.ListNew()
			if err != nil || len(pending) != 0 {
				t.Fatalf("cancelled resume remained queued: %+v, %v", pending, err)
			}
			f.input.screen = screens.busy
			if err := f.cmd.compact([]string{"worker", "--note", "replacement"}); err != nil {
				t.Fatal(err)
			}
			after, err = p.Read()
			if err != nil || after.Compaction == nil || after.Compaction.Status != "queued" || after.Compaction.Resume != (core.Message{Text: "replacement"}) {
				t.Fatalf("replacement request: %+v, %v", after.Compaction, err)
			}
		})
	}
}
