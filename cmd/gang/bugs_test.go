package main

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/store"
)

func TestStatusObservesCurrentActivity(t *testing.T) {
	for _, tc := range []struct {
		name   string
		before core.Activity
		screen []string
		want   core.Activity
	}{
		{"working after idle", core.Idle, []string{"Working (esc to interrupt)", "› "}, core.Busy},
		{"ready after uncertain delivery", core.Wedged, []string{"READY", "› "}, core.Idle},
		{"unrecognized surface", core.Idle, []string{"unrecognized screen"}, core.Unknown},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newStateFixture(t)
			a := f.add(t, "a", "worker", "codex")
			p, _ := f.run.team.Agent(a.ID)
			l, err := p.TryLock()
			if err != nil {
				t.Fatal(err)
			}
			a.Activity = tc.before
			a.Evidence = "context deadline exceeded"
			if err := l.Save(a); err != nil {
				t.Fatal(err)
			}
			l.Close()
			f.input.screen = screenWithText(tc.screen...)
			if err := f.cmd.status([]string{"worker", "--why"}); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(f.out.String(), "\t"+string(tc.want)+"\n") || strings.Contains(f.out.String(), "context deadline exceeded") {
				t.Fatalf("status: %s", f.out)
			}
		})
	}
}

func TestQueuedCompactionWaitsWithoutWedging(t *testing.T) {
	for _, budget := range []time.Duration{time.Millisecond, time.Second, time.Hour} {
		t.Run(budget.String(), func(t *testing.T) {
			f := newStateFixture(t)
			a := f.add(t, "a", "worker", "codex")
			start := f.cmd.now()
			a.Compaction = &core.Compaction{ID: "c", Status: "queued", Deadline: start.Add(budget)}
			// Inspect 1ns before, exactly at, and one full budget beyond expiry.
			for _, elapsed := range []time.Duration{budget - time.Nanosecond, budget, 2 * budget} {
				got, _ := core.Step(a, core.Event{Type: "deadline_checked", HitchID: a.ID, At: start.Add(elapsed)})
				if got.Activity != core.Idle || got.Compaction.Status != "queued" {
					t.Fatalf("elapsed=%s budget=%s: %+v", elapsed, budget, got)
				}
			}
		})
	}
}

// Drop reports and binds the session a submit witness or a status hook
// recorded after the last saved state, including the harness's confirmation
// of a requested resume session.
func TestDropReportsResumeIdentity(t *testing.T) {
	for _, c := range []struct{ requested, session, record string }{
		{"", "native-session", "witness"},
		{"native-session", "native-session", "witness"},
		{"native-session", "native-session", "status-hook"},
		{"", "", ""},
	} {
		requested, session := c.requested, c.session
		t.Run(requested+"/"+session+"/"+c.record, func(t *testing.T) {
			f := newStateFixture(t)
			a := f.add(t, "a", "worker", "codex")
			if requested != "" {
				saveAgent(t, f, a.ID, func(a *core.Agent) { a.Native.SessionID = requested })
			}
			p, _ := f.run.team.Agent(a.ID)
			switch c.record {
			case "witness":
				if err := p.WriteWitness(store.Witness{ID: "latest", SessionID: session, Transcript: session + ".jsonl", At: f.cmd.now()}); err != nil {
					t.Fatal(err)
				}
			case "status-hook":
				if err := p.WriteStatusHook(store.StatusHook{Kind: "turn-finished", At: f.cmd.now(), SessionID: session, Transcript: session + ".jsonl"}); err != nil {
					t.Fatal(err)
				}
			}
			if err := f.cmd.drop([]string{"worker"}); err != nil {
				t.Fatal(err)
			}
			want := "resume session: " + session
			if session == "" {
				want = "resume session: unknown"
			}
			if !strings.Contains(f.out.String(), want) {
				t.Fatalf("drop: %q, want %q", f.out, want)
			}
			var bound []core.Binding
			if session != "" {
				bound = []core.Binding{{SessionID: session, Collar: a.Collar}}
			}
			if got := sessionBindings(t, f, a.ID); !reflect.DeepEqual(got, bound) {
				t.Fatalf("bindings = %+v, want %+v", got, bound)
			}
		})
	}
}
