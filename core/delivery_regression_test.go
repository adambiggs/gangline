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
