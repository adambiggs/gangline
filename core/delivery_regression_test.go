package core

import "testing"

func TestCompactionCompletionReleasesContinuation(t *testing.T) {
	state := twoActiveHitches(t)
	compact := Compaction{ID: "c-1", HitchID: "worker-id", Resume: Message{Text: "continue"}, Deadline: testDeadline}
	state, _ = Step(state, CompactionRequested{At: testNow, Compaction: compact})
	state, _ = Step(state, CompactionCompleted{At: testNow, CompactionID: "c-1"})
	envelope := Envelope{ID: "resume-1", From: Sender{Kind: SenderSelfDeclared, Name: "compact"}, To: "worker", Message: compact.Resume, CreatedAt: testNow}
	state, effects := Step(state, SendRequested{At: testNow, Deadline: testDeadline, Envelope: envelope})
	if len(effects) != 1 || state.Deliveries[envelope.ID].Status != DeliveryDelivering {
		t.Fatalf("continuation stayed queued after completed compaction: %+v, effects=%v", state.Deliveries[envelope.ID], effects)
	}
}

func TestVerifiedInterruptLeavesHitchIdle(t *testing.T) {
	state := twoActiveHitches(t)
	state, _ = Step(state, TurnStarted{At: testNow, HitchID: "worker-id"})
	state, _ = Step(state, InterruptRequested{At: testNow, HitchID: "worker-id", Deadline: testDeadline})
	state, _ = Step(state, InterruptSucceeded{At: testNow, HitchID: "worker-id"})
	if state.Hitches["worker-id"].Activity != ActivityIdle {
		t.Fatalf("verified interrupt left activity %s", state.Hitches["worker-id"].Activity)
	}
}

func TestLiveDeliveryHasNoLifetimeDeadline(t *testing.T) {
	state := twoActiveHitches(t)
	state, _ = Step(state, TurnStarted{At: testNow, HitchID: "worker-id"})
	envelope := Envelope{ID: "forever", From: Sender{Kind: SenderSelfDeclared, Name: "operator"}, To: "worker", Message: Message{Text: "later"}, CreatedAt: testNow}
	state, _ = Step(state, SendRequested{At: testNow, Envelope: envelope})
	if state.Deliveries["forever"].Status != DeliveryQueued {
		t.Fatal("unbounded live delivery rejected")
	}
	state, _ = Step(state, DropRequested{At: testNow, HitchID: "worker-id", Deadline: testDeadline})
	state, _ = Step(state, DropSucceeded{At: testNow, HitchID: "worker-id"})
	if delivery := state.Deliveries["forever"]; delivery.Status != DeliveryFailed || delivery.Reason != "recipient was dropped" {
		t.Fatalf("drop did not visibly fail pending send: %+v", delivery)
	}
}

func TestUnboundedDeliverySurvivesEventCodec(t *testing.T) {
	event := SendRequested{At: testNow, Envelope: Envelope{ID: "forever", From: Sender{Kind: SenderSelfDeclared, Name: "operator"}, To: "worker", Message: Message{Text: "later"}, CreatedAt: testNow}}
	encoded, err := EncodeEvent(event)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeEvent(encoded)
	if err != nil || !decoded.(SendRequested).Deadline.IsZero() {
		t.Fatalf("unbounded deadline did not round trip: %v %v", decoded, err)
	}
}
