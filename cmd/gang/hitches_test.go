package main

import (
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
)

func TestStartupDeliveryRetriesWithoutAnOperatorTick(t *testing.T) {
	now := time.Date(2026, 9, 21, 8, 0, 0, 0, time.UTC)
	deadline := now.Add(startupDeliveryTimeout)
	id := core.EnvelopeID("startup-1")
	state := core.NewState(core.Team{ID: "team", Name: "team"})
	state.Hitches["h-1"] = core.Hitch{ID: "h-1", Name: "worker", Status: core.HitchActive, Activity: core.ActivityIdle}
	state.Deliveries[id] = core.Delivery{
		Envelope: core.Envelope{ID: id, To: "worker", Message: core.Message{Text: "assignment"}},
		Status:   core.DeliveryQueued, Deadline: deadline, Reason: "another surface owns input",
	}

	// A supplied observation is the fake clock: the production retry margin is
	// 100ms against a five-minute startup budget, while the gate budget is 90s.
	observations := make(chan time.Time, 1)
	observations <- now.Add(startupRetryInterval)
	close(observations)
	attempts := 0
	got, err := awaitStartupDelivery(state, id, observations, func() (core.State, error) {
		attempts++
		delivery := state.Deliveries[id]
		delivery.Status = core.DeliveryDelivered
		state.Deliveries[id] = delivery
		return state, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if attempts != 1 || got.Deliveries[id].Status != core.DeliveryDelivered {
		t.Fatalf("attempts = %d, delivery = %#v", attempts, got.Deliveries[id])
	}
}

func TestStartupDeliveryUsesAHumanScaleDeadline(t *testing.T) {
	if startupDeliveryTimeout <= deliveryTimeout {
		t.Fatalf("startup deadline = %s, ordinary delivery deadline = %s", startupDeliveryTimeout, deliveryTimeout)
	}
}
