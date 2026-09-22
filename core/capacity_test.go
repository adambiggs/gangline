package core

import (
	"fmt"
	"testing"
	"time"
)

func TestCapacityBoundaryReleasesQueuedUserInput(t *testing.T) {
	state := busyHitch(t)
	envelope := Envelope{ID: "capacity-user", From: Sender{Kind: SenderSelfDeclared, Name: "operator"}, To: "worker", Message: Message{Text: "continue the work"}, CreatedAt: testNow}
	state = apply(t, state, SendRequested{At: testNow, Envelope: envelope})
	event, err := DecodeEvent([]byte(fmt.Sprintf(`{"type":"capacity_detected","at":%q,"hitch_id":"worker-id","fingerprint":"failed-turn-one","evidence":"terminal native capacity error","deadline":%q}`, testNow.Format("2006-01-02T15:04:05Z07:00"), testDeadline.Format("2006-01-02T15:04:05Z07:00"))))
	if err != nil {
		t.Fatalf("native terminal capacity boundary cannot be recorded: %v", err)
	}
	state, effects := Step(state, event)
	if state.Deliveries[envelope.ID].Status != DeliveryDelivering || len(effects) != 1 {
		t.Fatalf("terminal capacity left queued user input stranded: activity=%s delivery=%+v effects=%v", state.Hitches["worker-id"].Activity, state.Deliveries[envelope.ID], effects)
	}
}

func capacityState(t *testing.T) State {
	t.Helper()
	return apply(t, busyHitch(t), CapacityDetected{At: testNow, HitchID: "worker-id", Fingerprint: "first", Evidence: "terminal native capacity error", Deadline: testDeadline})
}

func TestCapacitySchedulePersistsAcrossRepeatedAndFreshFailures(t *testing.T) {
	state := capacityState(t)
	first := state.Hitches["worker-id"].Capacity
	state = apply(t, state, CapacityDetected{At: testNow.Add(time.Second), HitchID: "worker-id", Fingerprint: "first", Evidence: first.Evidence, Deadline: testDeadline.Add(time.Hour)})
	if got := state.Hitches["worker-id"].Capacity; got != first {
		t.Fatalf("repeated observation changed schedule: %+v -> %+v", first, got)
	}
	state = apply(t, state, CapacityRetryRequested{At: first.NextAt, HitchID: "worker-id", Fingerprint: "first"})
	id := state.Hitches["worker-id"].Capacity.EnvelopeID
	if len(state.Deliveries) != 1 || state.Deliveries[id].Status != DeliveryDelivering {
		t.Fatalf("missing attributed continuation: %+v", state)
	}
	state, effects := Step(state, CapacityRetryRequested{At: first.NextAt, HitchID: "worker-id", Fingerprint: "first"})
	if len(state.Deliveries) != 1 || len(effects) != 1 {
		t.Fatalf("concurrent observation duplicated continuation: %+v effects=%v", state.Deliveries, effects)
	}
	assertNoCapacityInput(t, effects)
	state = apply(t, state, DeliverySucceeded{At: first.NextAt, EnvelopeID: id})
	at := first.NextAt.Add(time.Second)
	state = apply(t, state, CapacityDetected{At: at, HitchID: "worker-id", Fingerprint: "second", Evidence: first.Evidence, Deadline: testDeadline.Add(time.Hour)})
	next := state.Hitches["worker-id"].Capacity
	if next.Deadline != first.Deadline || next.NextAt != at.Add(200*time.Millisecond) || next.Attempts != 1 {
		t.Fatalf("fresh failure reset episode or backoff: %+v", next)
	}
	state = apply(t, state, CapacityRetryRequested{At: next.NextAt, HitchID: "worker-id", Fingerprint: "second"})
	if len(state.Deliveries) != 2 || state.Hitches["worker-id"].Capacity.EnvelopeID == id {
		t.Fatalf("fresh failure reused submitted user input: %+v", state.Deliveries)
	}
}

func TestCapacityExpiryCancelsOnlySafelyQueuedRecovery(t *testing.T) {
	state := capacityState(t)
	state = apply(t, state, CapacityRetryRequested{At: testNow.Add(100 * time.Millisecond), HitchID: "worker-id", Fingerprint: "first"})
	id := state.Hitches["worker-id"].Capacity.EnvelopeID
	state = apply(t, state, DeliveryDeferred{At: testNow.Add(100 * time.Millisecond), EnvelopeID: id, Reason: "composer changed before input"})
	user := Envelope{ID: "user", From: Sender{Kind: SenderSelfDeclared, Name: "operator"}, To: "worker", Message: Message{Text: "later user message"}, CreatedAt: testNow}
	state = apply(t, state, SendRequested{At: testNow, Envelope: user, NotBefore: testDeadline.Add(time.Hour)})
	state = apply(t, state, CapacityExpired{At: testDeadline, HitchID: "worker-id"})
	if state.Deliveries[id].Status != DeliveryCancelled || state.Deliveries[user.ID].Status != DeliveryQueued || state.Hitches["worker-id"].Activity != ActivityWedged {
		t.Fatalf("expiry changed user lifetime or failed to mark attention: %+v", state)
	}
	_, effects := Step(state, CapacityRetryRequested{At: testDeadline, HitchID: "worker-id", Fingerprint: "first"})
	assertNoCapacityInput(t, effects)
}

func TestCapacityUnknownInputNeverAutomaticallyRetries(t *testing.T) {
	state := capacityState(t)
	at := testNow.Add(100 * time.Millisecond)
	state = apply(t, state, CapacityRetryRequested{At: at, HitchID: "worker-id", Fingerprint: "first"})
	id := state.Hitches["worker-id"].Capacity.EnvelopeID
	state = apply(t, state, DeliveryUnverifiedEvent{At: at, EnvelopeID: id, Evidence: "native witness missing"})
	state, effects := Step(state, CapacityRetryRequested{At: at.Add(time.Second), HitchID: "worker-id", Fingerprint: "first"})
	if state.Deliveries[id].Status != DeliveryUnverified || len(state.Deliveries) != 1 {
		t.Fatalf("unknown input was made retryable: %+v", state.Deliveries)
	}
	for _, effect := range effects {
		if _, typed := effect.(DeliverEnvelope); typed {
			t.Fatal("unknown input was retyped")
		}
	}
}

func assertNoCapacityInput(t *testing.T, effects []Effect) {
	t.Helper()
	for _, effect := range effects {
		if _, typed := effect.(DeliverEnvelope); typed {
			t.Fatal("unsafe capacity retry emitted native input")
		}
	}
}

func TestCapacityRetryRequiresScheduleAndReusesOnlyPreInputDeferral(t *testing.T) {
	state := capacityState(t)
	_, effects := Step(state, CapacityRetryRequested{At: testNow, HitchID: "worker-id", Fingerprint: "first"})
	assertNoCapacityInput(t, effects)
	at := state.Hitches["worker-id"].Capacity.NextAt
	state = apply(t, state, CapacityRetryRequested{At: at, HitchID: "worker-id", Fingerprint: "first"})
	id := state.Hitches["worker-id"].Capacity.EnvelopeID
	state = apply(t, state, DeliveryDeferred{At: at, EnvelopeID: id, Reason: "input still occupied"})
	state = apply(t, state, CapacityRetryRequested{At: state.Hitches["worker-id"].Capacity.NextAt, HitchID: "worker-id", Fingerprint: "first"})
	if len(state.Deliveries) != 1 || state.Hitches["worker-id"].Capacity.EnvelopeID != id || state.Deliveries[id].Status != DeliveryDelivering {
		t.Fatalf("safe preinput retry changed continuation: %+v", state.Deliveries)
	}
}

func TestCapacityRecoveryClearsOnlyOnNativeProgress(t *testing.T) {
	state := capacityState(t)
	state = apply(t, state, CapacityRetryRequested{At: testNow.Add(100 * time.Millisecond), HitchID: "worker-id", Fingerprint: "first"})
	id := state.Hitches["worker-id"].Capacity.EnvelopeID
	state = apply(t, state, DeliverySucceeded{At: testNow.Add(100 * time.Millisecond), EnvelopeID: id})
	if state.Hitches["worker-id"].Capacity.Fingerprint == "" {
		t.Fatal("submission alone claimed provider recovery")
	}
	for _, event := range []Event{CapacityCleared{At: testNow.Add(time.Second), HitchID: "worker-id"}, TurnBoundaryReached{At: testNow.Add(time.Second), HitchID: "worker-id"}} {
		next := apply(t, state, event)
		if next.Hitches["worker-id"].Capacity.Fingerprint != "" {
			t.Fatalf("native progress did not clear recovery: %+v", event)
		}
	}
}

func TestCapacityEventsRoundTrip(t *testing.T) {
	for _, event := range []Event{
		CapacityDetected{At: testNow, HitchID: "worker-id", Fingerprint: "one", Evidence: "capacity", Deadline: testDeadline},
		CapacityRetryRequested{At: testNow, HitchID: "worker-id", Fingerprint: "one"},
		CapacityExpired{At: testNow, HitchID: "worker-id"},
		CapacityCleared{At: testNow, HitchID: "worker-id"},
	} {
		data, err := EncodeEvent(event)
		if err != nil {
			t.Fatal(err)
		}
		got, err := DecodeEvent(data)
		if err != nil || got != event {
			t.Fatalf("round trip %T = %+v, %v", event, got, err)
		}
	}
	for attempt, want := range []time.Duration{100 * time.Millisecond, 200 * time.Millisecond, 400 * time.Millisecond, 800 * time.Millisecond} {
		if got := RetryDelay(attempt); got != want {
			t.Fatalf("attempt %d: %v", attempt, got)
		}
	}
	if got := RetryDelay(1000000); got != 30*time.Second {
		t.Fatalf("retry cap = %v", got)
	}
}
