package main

import (
	"strings"
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
)

func TestOccupiedComposerIsNotNativeWork(t *testing.T) {
	for _, tc := range []struct {
		name, collar string
		screen       []string
		want         core.Activity
	}{
		{"finished Codex turn", "codex", []string{"Worked for 18m · done", "› unsubmitted message", "  continuation"}, core.Blocked},
		{"finished Claude turn", "claude-code", []string{"────────", "❯ unsubmitted message", "────────"}, core.Blocked},
		{"working with draft", "codex", []string{"Working (esc to interrupt)", "› draft for later"}, core.Busy},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newStateFixture(t)
			a := f.add(t, "a", "worker", tc.collar)
			f.input.screen = screenWithText(tc.screen...)
			if err := f.cmd.status([]string{"worker", "--why"}); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(f.out.String(), "\t"+string(tc.want)+"\n") {
				t.Fatalf("wrong activity: %q", f.out)
			}
			if tc.want == core.Blocked && !strings.Contains(f.out.String(), "unsubmitted input") {
				t.Fatalf("missing composer reason: %q", f.out)
			}
			f.out.Reset()
			if err := f.cmd.roster([]string{"--porcelain"}); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(f.out.String(), "\t"+string(tc.want)+"\t") {
				t.Fatalf("wrong roster: %q", f.out)
			}
			p, _ := f.run.team.Agent(a.ID)
			got, err := p.Read()
			if err != nil || got.Activity != tc.want || f.input.submits != 0 || f.input.pasted != "" {
				t.Fatalf("observation changed input: activity=%s submits=%d pasted=%q err=%v", got.Activity, f.input.submits, f.input.pasted, err)
			}
		})
	}
}

func TestPendingCompactionDoesNotHideUnsubmittedInput(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "codex")
	a.Compaction = &core.Compaction{ID: "c", Status: "submitted", StartedAt: f.cmd.now(), Deadline: f.cmd.now().Add(time.Hour)}
	p, _ := f.run.team.Agent(a.ID)
	l, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Save(a); err != nil {
		t.Fatal(err)
	}
	l.Close()
	f.input.screen = screenWithText("› unsubmitted message")
	if err := f.cmd.status([]string{"worker", "--why"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(f.out.String(), "\tblocked\n") || !strings.Contains(f.out.String(), "unsubmitted input") || f.input.submits != 0 {
		t.Fatalf("compaction hid the composer: %q", f.out)
	}
}
