package main

import (
	"errors"
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
)

func TestBootObservationPrecedesDeadlineFailure(t *testing.T) {
	deadline := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	effect := core.AwaitBoot{HitchID: "h-1", Pane: "%1", Deadline: deadline}

	ready := bootObservationOutcome(deadline.Add(time.Minute), effect, harness.Startup{State: harness.StartupReady}, nil)
	if event, ok := ready.(core.HitchReady); !ok || event.HitchID != effect.HitchID {
		t.Fatalf("ready observation = %#v, want HitchReady", ready)
	}

	if prompt := bootObservationOutcome(deadline.Add(time.Minute), effect, harness.Startup{State: harness.StartupTrustRequired}, nil); prompt != nil {
		t.Fatalf("operator prompt observation = %#v, want boot to remain pending", prompt)
	}

	if pending := bootObservationOutcome(deadline.Add(-time.Second), effect, harness.Startup{}, errors.New("not observable")); pending != nil {
		t.Fatalf("pre-deadline observation = %#v, want boot to remain pending", pending)
	}

	timedOut := bootObservationOutcome(deadline.Add(time.Minute), effect, harness.Startup{}, errors.New("not observable"))
	if event, ok := timedOut.(core.OperationTimedOut); !ok || event.Operation != core.TimeoutBoot {
		t.Fatalf("expired unreadable observation = %#v, want boot timeout", timedOut)
	}
}

func TestWindowTitleProjectsRecordedHitchState(t *testing.T) {
	tests := []struct {
		name     string
		status   core.HitchStatus
		activity core.HitchActivity
		want     string
	}{
		{name: "booting", status: core.HitchBooting, activity: core.ActivityUnknown, want: "?worker?"},
		{name: "idle", status: core.HitchActive, activity: core.ActivityIdle, want: "~worker~"},
		{name: "working", status: core.HitchActive, activity: core.ActivityBusy, want: "-worker-"},
		{name: "blocked", status: core.HitchActive, activity: core.ActivityBlocked, want: "!worker!"},
		{name: "wedged", status: core.HitchActive, activity: core.ActivityWedged, want: "!worker!"},
		{name: "failed", status: core.HitchFailed, activity: core.ActivityUnknown, want: "!worker!"},
		{name: "dropped", status: core.HitchDropped, activity: core.ActivityUnknown, want: ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := windowTitle(core.Hitch{Name: "worker", Status: test.status, Activity: test.activity})
			if got != test.want {
				t.Fatalf("window title = %q, want %q", got, test.want)
			}
		})
	}
}

func TestLegacyDuplicateLookupPrefersActiveAndDropPrefersFailed(t *testing.T) {
	state := core.NewState(core.Team{ID: "team", Name: "team"})
	state.Hitches["failed"] = core.Hitch{ID: "failed", Name: "worker", Status: core.HitchFailed}
	state.Hitches["active"] = core.Hitch{ID: "active", Name: "worker", Status: core.HitchActive}

	if hitch, ok := hitchByName(state, "worker"); !ok || hitch.ID != "active" {
		t.Fatalf("ordinary lookup = %#v, %v; want active generation", hitch, ok)
	}
	if hitch, ok := dropCandidateByName(state, "worker"); !ok || hitch.ID != "failed" {
		t.Fatalf("drop lookup = %#v, %v; want failed generation", hitch, ok)
	}
}
