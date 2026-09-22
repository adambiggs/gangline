package core

import "testing"

func TestBlockedObservationDoesNotRequeueStartedInput(t *testing.T) {
	for _, clearFirst := range []bool{false, true} {
		state := deliveringHitch(t)
		envelope := state.Deliveries["e-1"].Envelope
		state, _ = Step(state, DeliveryInputStarted{At: testNow, EnvelopeID: envelope.ID})
		state, _ = Step(state, BlockedDetected{At: testNow, HitchID: "worker-id", Evidence: "hooks need review"})
		if state.Deliveries[envelope.ID].Status != DeliveryDelivering {
			t.Fatal("observer requeued input already started")
		}
		if clearFirst {
			state, _ = Step(state, BlockedCleared{At: testNow, HitchID: "worker-id"})
		}
		state, _ = Step(state, DeliveryUnverifiedEvent{At: testNow, EnvelopeID: envelope.ID, Evidence: "review appeared after paste"})
		state, effects := Step(state, DeliveryRetryRequested{At: testNow, EnvelopeID: envelope.ID})
		if state.Deliveries[envelope.ID].Status != DeliveryUnverified {
			t.Fatal("unknown input became retryable")
		}
		for _, effect := range effects {
			if _, ok := effect.(DeliverEnvelope); ok {
				t.Fatal("unknown input retyped")
			}
		}
	}
}
