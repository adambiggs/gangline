package main

import (
	"context"
	"testing"
	"testing/synctest"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/substrate"
)

type hookReviewBackend struct {
	witnessBackend
	before bool
}

func (b *hookReviewBackend) Capture(ctx context.Context, pane substrate.PaneID) (substrate.Screen, error) {
	if b.before || b.text != "" {
		return screenWithText("Hooks", "2 hooks need review before they can run", "› Stop", "Enter to review"), nil
	}
	return b.witnessBackend.Capture(ctx, pane)
}

func TestHookReviewBeforeInputStaysBlockedAndQueued(t *testing.T) {
	cmd, run, state, envelope := witnessFixture(t)
	backend := &hookReviewBackend{witnessBackend: witnessBackend{cmd: cmd}, before: true}
	event, err := run.deliver(state, backend, core.DeliverEnvelope{Envelope: envelope, Pane: "%1"})
	if err != nil {
		t.Fatal(err)
	}
	state, _ = core.Step(state, event)
	if state.Hitches["h-1"].Activity != core.ActivityBlocked || state.Deliveries[envelope.ID].Status != core.DeliveryQueued {
		t.Fatalf("review lost: event %#v, hitch %#v", event, state.Hitches["h-1"])
	}
	if backend.text != "" || backend.submits != 0 {
		t.Fatal("input entered hook review")
	}
}

func TestHookReviewAfterPasteIsUnverifiedAndBlocked(t *testing.T) {
	cmd, run, state, envelope := witnessFixture(t)
	synctest.Test(t, func(t *testing.T) {
		backend := &hookReviewBackend{witnessBackend: witnessBackend{cmd: cmd}}
		event, err := run.deliver(state, backend, core.DeliverEnvelope{Envelope: envelope, Pane: "%1"})
		if err != nil {
			t.Fatal(err)
		}
		state, _ = core.Step(state, event)
		if state.Hitches["h-1"].Activity != core.ActivityBlocked || state.Deliveries[envelope.ID].Status != core.DeliveryUnverified {
			t.Fatalf("post-paste review misclassified: event %#v, hitch %#v", event, state.Hitches["h-1"])
		}
		if backend.submits != 0 {
			t.Fatal("Enter entered hook review")
		}
		state, _ = core.Step(state, core.BlockedCleared{HitchID: "h-1"})
		if state.Deliveries[envelope.ID].Status != core.DeliveryUnverified {
			t.Fatal("review clearance made unknown input retryable")
		}
	})
}

func TestMessageMentioningHookReviewIsSubmitted(t *testing.T) {
	cmd, run, state, envelope := witnessFixture(t)
	envelope.Message.Text = "Investigate Hooks need review"
	synctest.Test(t, func(t *testing.T) {
		backend := &witnessBackend{cmd: cmd}
		event, err := run.deliver(state, backend, core.DeliverEnvelope{Envelope: envelope, Pane: "%1"})
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := event.(core.DeliverySucceeded); !ok {
			t.Fatalf("ordinary body mistaken for review: %#v", event)
		}
	})
}
