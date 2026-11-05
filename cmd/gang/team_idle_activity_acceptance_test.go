//go:build acceptance

package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/store"
)

func (f *idleFixture) activity(t *testing.T, activity core.Activity) {
	t.Helper()
	p, _ := f.run.team.Agent(f.worker.ID)
	l, err := p.LockAgent()
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	a, err := p.Read()
	if err != nil {
		t.Fatal(err)
	}
	if err := f.run.apply(l, &a, core.Event{Type: "activity_observed", Activity: activity, Reason: "observed fixture activity"}); err != nil {
		t.Fatal(err)
	}
	f.worker = a
}

// A fake clock covers the full 30 minutes in 20-second observations, including
// a one-nanosecond margin below the boundary. No wall-clock waiting is involved.
func (f *idleFixture) thirtyMinutes(t *testing.T, previous core.EnvelopeID) *store.IdleNotice {
	t.Helper()
	f.reserve(t)
	for range 89 {
		f.advance(t, 20*time.Second)
		if n := f.reserve(t); n != nil && n.Envelope.ID != previous {
			t.Fatal("notice before 30-minute threshold")
		}
	}
	f.advance(t, 20*time.Second-time.Nanosecond)
	if n := f.reserve(t); n != nil && n.Envelope.ID != previous {
		t.Fatal("notice one nanosecond before threshold")
	}
	f.advance(t, time.Nanosecond)
	return f.reserve(t)
}

func TestTeamIdleObservedActivityWaitsThirtyMinutes(t *testing.T) {
	f := newIdleFixture(t)
	f.run.settings.TeamIdleMinDuration = 30 * time.Minute
	f.reserve(t)
	// Autonomous work, with no Gangline message; entire turn is between sweeps.
	f.activity(t, core.Busy)
	f.advance(t, time.Second)
	f.activity(t, core.Idle)
	if n := f.thirtyMinutes(t, ""); n == nil {
		t.Fatal("observed activity did not arm notice")
	}
}

func TestTeamIdleUnchangedIdleAndMessageDoNotRearm(t *testing.T) {
	f := newIdleFixture(t)
	f.run.settings.TeamIdleMinDuration = 30 * time.Minute
	f.env["GANG_TEAM_IDLE_MIN_DURATION"] = "30m"
	f.reserve(t)
	// Publication alone has no observed native activity. Even a settled message
	// must not manufacture an active period.
	f.publication(t, "publication-only", time.Time{})
	if n := f.thirtyMinutes(t, ""); n != nil {
		t.Fatal("message alone armed idle notice")
	}
	f.activity(t, core.Busy)
	f.advance(t, time.Second)
	f.activity(t, core.Idle)
	n := f.thirtyMinutes(t, "")
	if n == nil {
		t.Fatal("missing first notice")
	}
	l, err := f.run.team.LockIdle()
	if err != nil {
		t.Fatal(err)
	}
	s, err := l.Read()
	if err != nil {
		t.Fatal(err)
	}
	s.Notice.Outcome = "delivered"
	if err := l.Save(s); err != nil {
		t.Fatal(err)
	}
	l.Close()
	f.run, err = f.cmd.runtime()
	if err != nil {
		t.Fatal(err)
	}
	f.publication(t, "another-publication-only", time.Time{})
	if next := f.thirtyMinutes(t, n.Envelope.ID); next == nil || next.Envelope.ID != n.Envelope.ID {
		t.Fatal("idle/restart/message repeated notice")
	}
}

func TestTeamIdleActivityRearmsOnceAfterFullTimeout(t *testing.T) {
	f := newIdleFixture(t)
	f.run.settings.TeamIdleMinDuration = 30 * time.Minute
	f.reserve(t)
	f.activity(t, core.Busy)
	f.advance(t, time.Second)
	f.activity(t, core.Idle)
	first := f.thirtyMinutes(t, "")
	if first == nil {
		t.Fatal("missing first notice")
	}
	l, err := f.run.team.LockIdle()
	if err != nil {
		t.Fatal(err)
	}
	s, err := l.Read()
	if err != nil {
		t.Fatal(err)
	}
	s.Notice.Outcome = "delivered"
	if err := l.Save(s); err != nil {
		t.Fatal(err)
	}
	l.Close()
	f.advance(t, time.Second)
	f.activity(t, core.Busy)
	f.advance(t, time.Second)
	f.activity(t, core.Idle)
	next := f.thirtyMinutes(t, first.Envelope.ID)
	if next == nil || next.Envelope.ID == first.Envelope.ID {
		t.Fatal("new observed active period did not rearm")
	}
	if again := f.thirtyMinutes(t, next.Envelope.ID); again == nil || again.Envelope.ID != next.Envelope.ID {
		t.Fatal("same idle period repeated")
	}
}

// A failed activity-latch write must leave the observation retryable.
func TestTeamIdleActivityFailureDoesNotConsumeObservation(t *testing.T) {
	f := newIdleFixture(t)
	f.reserve(t)
	path := filepath.Join(f.run.team.Directory, "idle.json")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	p, _ := f.run.team.Agent(f.worker.ID)
	l, err := p.LockAgent()
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	a, err := p.Read()
	if err != nil {
		t.Fatal(err)
	}
	before := a.Activity
	if err := f.run.apply(l, &a, core.Event{Type: "activity_observed", Activity: core.Busy}); err == nil {
		t.Fatal("expected idle-state failure")
	}
	after, err := p.Read()
	if err != nil {
		t.Fatal(err)
	}
	if after.Activity != before {
		t.Fatal("activity consumed before its rearm latch was saved")
	}
}
