package main

import (
	"strings"
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/store"
)

func TestIdleEpisodeIdentity(t *testing.T) {
	lead := store.IdleMember{ID: "lead", Generation: "l"}
	worker := store.IdleMember{ID: "worker", Generation: "one", Work: 5}
	var s store.IdleState
	observe := func() { advanceIdleEpisode(&s, lead, []store.IdleMember{worker}) }
	observe()
	if s.Armed {
		t.Fatal("initial idle observation armed an alert")
	}
	worker.Work++
	observe()
	if !s.Armed {
		t.Fatal("new identified work did not arm")
	}
	s.Armed = false // one reserved notice
	for range 3 {
		observe()
		if s.Armed {
			t.Fatal("unchanged work rearmed")
		}
	}
	worker.Work-- // older work evidence cannot rearm
	observe()
	if s.Armed {
		t.Fatal("older work rearmed")
	}
	worker.Generation = "two"
	worker.Work = 20
	observe()
	if s.Armed {
		t.Fatal("replacement manufactured completion")
	}
	worker.Work++
	observe()
	if !s.Armed {
		t.Fatal("work after replacement did not rearm")
	}
	advanceIdleEpisode(&s, lead, nil)
	if s.Armed {
		t.Fatal("drop manufactured completion")
	}
}

func TestIdleObservationBoundaries(t *testing.T) {
	now := time.Date(2026, 11, 5, 1, 2, 3, 0, time.UTC)
	good := core.Agent{Status: core.Active, Activity: core.Idle, Pane: "%1", StatusProbeAt: now}
	cases := []struct {
		name   string
		change func(*core.Agent)
	}{
		{"busy", func(a *core.Agent) { a.Activity = core.Busy }},
		{"unknown", func(a *core.Agent) { a.Activity = core.Unknown }},
		{"blocked", func(a *core.Agent) { a.Activity = core.Blocked }},
		{"failed", func(a *core.Agent) { a.Status = core.Failed }},
		{"missing", func(a *core.Agent) { a.StatusProbeAt = time.Time{} }},
		{"stale", func(a *core.Agent) { a.StatusProbeAt = now.Add(-statusHookFreshness) }},
		{"future", func(a *core.Agent) { a.Native.HookAt = now.Add(time.Second) }},
		{"input", func(a *core.Agent) { a.Input = &core.InputIntent{} }},
		{"outage", func(a *core.Agent) { a.InputOutage = &core.InputOutage{} }},
		{"draft", func(a *core.Agent) { a.Evidence = heldInputEvidence }},
		{"interrupt", func(a *core.Agent) { a.InterruptDeadline = now }},
		{"compaction", func(a *core.Agent) { a.Compaction = &core.Compaction{Status: "unverified"} }},
		{"native-failure", func(a *core.Agent) { a.Native.TurnFailure = "failed" }},
		{"mailbox", func(a *core.Agent) { a.Pane = "" }},
	}
	if !idleObservation(good, now) {
		t.Fatal("fresh idle observation rejected")
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := good
			tc.change(&a)
			if idleObservation(a, now) {
				t.Fatal("unavailable observation accepted")
			}
		})
	}
	for _, turn := range []string{"new", "old", "new"} {
		a := good
		a.Native.TurnID = turn
		if !idleObservation(a, now) {
			t.Fatal("turn IDs changed observation policy")
		}
	}
}

func TestIdleNoticeHonesty(t *testing.T) {
	at := time.Date(2026, 11, 5, 1, 0, 0, 0, time.UTC)
	text := idleNoticeText("team", []store.IdleMember{{ID: "c", Name: "claude"}, {ID: "x", Name: "codex"}}, at, at.Add(time.Second), 1, at.Add(time.Hour))
	for _, want := range []string{"all 2 registered workers", "claude (c)", "codex (x)", "2026-11-05T01:00:00Z–2026-11-05T01:00:01Z", "quiet gaps can look idle", "native child/background work may still be running", "does not mean assignments are complete", "1 pending; next due 2026-11-05T02:00:00Z"} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in %s", want, text)
		}
	}
	for _, outcome := range []string{"", "accepted", "unverified"} {
		if idleNoticeSettled(&store.IdleNotice{Outcome: outcome}) {
			t.Errorf("uncertain notice %q treated as settled", outcome)
		}
	}
}
