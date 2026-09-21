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
