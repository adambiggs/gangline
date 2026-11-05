package main

import (
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
	if s.Armed {
		t.Fatal("publication alone armed a notice")
	}
	at := time.Date(2026, 11, 5, 1, 0, 0, 0, time.UTC)
	a := core.Agent{ID: worker.ID, Status: core.Active, Registration: core.PaneRegistration{Generation: worker.Generation}}
	if !armIdleActivity(&s, a, at) || !s.Armed {
		t.Fatal("observed worker activity did not arm")
	}
	s.Armed = false
	s.Notice = &store.IdleNotice{Envelope: core.Envelope{CreatedAt: at}}
	observe()
	if s.Armed || armIdleActivity(&s, a, at.Add(-time.Second)) || armIdleActivity(&s, a, at) {
		t.Fatal("old evidence rearmed")
	}
	worker.Generation = "two"
	worker.Work = 20
	observe()
	if s.Armed || armIdleActivity(&s, a, at.Add(time.Second)) {
		t.Fatal("replacement manufactured completion")
	}
	a.Registration.Generation = "two"
	if !armIdleActivity(&s, a, at.Add(time.Second)) || !s.Armed {
		t.Fatal("new worker activity did not rearm")
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

func TestIdleNoticeText(t *testing.T) {
	at := time.Date(2026, 11, 5, 1, 0, 0, 0, time.UTC)
	text := idleNoticeText([]store.IdleMember{{ID: "c", Name: "claude"}, {ID: "x", Name: "codex"}}, at)
	if want := "All 2 workers idle since 2026-11-05T01:00:00Z: claude, codex."; text != want {
		t.Errorf("notice = %q, want %q", text, want)
	}

	for _, outcome := range []string{"", "accepted", "unverified"} {
		if idleNoticeSettled(&store.IdleNotice{Outcome: outcome}) {
			t.Errorf("uncertain notice %q treated as settled", outcome)
		}
	}
}

func TestIdleWindowThresholdAndContinuity(t *testing.T) {
	at := time.Date(2026, 11, 5, 1, 0, 0, 0, time.UTC)
	sample := func(offset time.Duration) store.IdleWindow {
		return store.IdleWindow{Since: at.Add(offset), Last: at.Add(offset), Minimum: time.Minute,
			Lead: store.IdleMember{ID: "lead"}, Members: []store.IdleWindowMember{{Member: store.IdleMember{ID: "worker", Work: 1}, ChangedAt: at, ScreenSince: at, HookSequence: 4}}}
	}
	first, mature := advanceIdleWindow(nil, sample(0), true)
	if mature {
		t.Fatal("first observation matured")
	}
	window := first
	for _, offset := range []time.Duration{20 * time.Second, 40 * time.Second, time.Minute - time.Nanosecond} {
		window, mature = advanceIdleWindow(window, sample(offset), true)
		if mature {
			t.Fatalf("matured early at %s", offset)
		}
	}
	window, mature = advanceIdleWindow(window, sample(time.Minute), true)
	if !mature || !window.Since.Equal(at) {
		t.Fatal("exact threshold missed")
	}
	if _, mature := advanceIdleWindow(window, sample(65*time.Second), true); !mature {
		t.Fatal("above threshold missed")
	}
	for _, tc := range []struct {
		name  string
		edit  func(*store.IdleWindow)
		ready bool
	}{
		{"busy", func(*store.IdleWindow) {}, false},
		{"clock-back", func(w *store.IdleWindow) { w.Last = at.Add(-time.Second) }, true},
		{"gap-at-freshness", func(w *store.IdleWindow) { w.Last = at.Add(90 * time.Second) }, true},
		{"minimum", func(w *store.IdleWindow) { w.Minimum = time.Second }, true},
		{"lead", func(w *store.IdleWindow) { w.Lead.Generation = "new" }, true},
		{"member", func(w *store.IdleWindow) { w.Members[0].Member.Generation = "new" }, true},
		{"work", func(w *store.IdleWindow) { w.Members[0].Member.Work++ }, true},
		{"hook", func(w *store.IdleWindow) { w.Members[0].HookSequence++ }, true},
		{"state", func(w *store.IdleWindow) { w.Members[0].ChangedAt = at.Add(time.Second) }, true},
		{"screen", func(w *store.IdleWindow) { w.Members[0].ScreenSince = at.Add(time.Second) }, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			current := sample(65 * time.Second)
			tc.edit(&current)
			next, mature := advanceIdleWindow(window, current, tc.ready)
			if mature {
				t.Fatal("changed evidence matured previous window")
			}
			if next != nil && !next.Since.Equal(current.Since) {
				t.Fatal("did not restart delay")
			}
		})
	}
	tiny := sample(0)
	tiny.Minimum = time.Nanosecond
	first, mature = advanceIdleWindow(nil, tiny, true)
	if mature {
		t.Fatal("tiny minimum skipped first sweep")
	}
	tiny.Last = at.Add(time.Nanosecond)
	if _, mature = advanceIdleWindow(first, tiny, true); !mature {
		t.Fatal("tiny minimum not supported")
	}
}
