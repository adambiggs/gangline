package core

import (
	"encoding/json"
	"testing"
)

func completionWithContinuation(t *testing.T) CompactionCompleted {
	t.Helper()
	data, err := json.Marshal(map[string]any{
		"at": testNow, "compaction_id": "c-1",
		"continuation": Envelope{ID: "resume-c-1", From: Sender{Kind: SenderSelfDeclared, Name: "compact"}, To: "worker", Message: Message{Text: "continue"}, CreatedAt: testNow},
	})
	if err != nil {
		t.Fatal(err)
	}
	var event CompactionCompleted
	if err := json.Unmarshal(data, &event); err != nil {
		t.Fatal(err)
	}
	return event
}

func TestCompactionCompletionAtomicallyPersistsContinuation(t *testing.T) {
	state := twoActiveHitches(t)
	state, _ = Step(state, CompactionRequested{At: testNow, Compaction: Compaction{ID: "c-1", HitchID: "worker-id", Resume: Message{Text: "continue"}, Deadline: testDeadline}})
	completion := completionWithContinuation(t)
	state, effects := Step(state, completion)
	if len(state.Deliveries) != 1 || state.Deliveries["resume-c-1"].Status != DeliveryDelivering || len(effects) != 1 {
		t.Fatalf("completion lost its continuation: deliveries=%v effects=%v", state.Deliveries, effects)
	}
	state, _ = Step(state, completion)
	if len(state.Deliveries) != 1 || len(state.DeliveryOrder) != 1 {
		t.Fatalf("duplicate completion duplicated continuation: %v", state.DeliveryOrder)
	}
	encoded, err := EncodeEvent(completion)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeEvent(encoded)
	if err != nil {
		t.Fatal(err)
	}
	replayed := twoActiveHitches(t)
	replayed, _ = Step(replayed, CompactionRequested{At: testNow, Compaction: Compaction{ID: "c-1", HitchID: "worker-id", Resume: Message{Text: "continue"}, Deadline: testDeadline}})
	replayed, _ = Step(replayed, decoded)
	if len(replayed.Deliveries) != 1 || replayed.Deliveries["resume-c-1"].Status != DeliveryDelivering {
		t.Fatalf("completion-only replay lost continuation: %v", replayed.Deliveries)
	}
}

func TestCompactionLegacyCompletionDoesNotSynthesizeContinuation(t *testing.T) {
	state := twoActiveHitches(t)
	state, _ = Step(state, CompactionRequested{At: testNow, Compaction: Compaction{ID: "c-1", HitchID: "worker-id", Resume: Message{Text: "continue"}, Deadline: testDeadline}})
	state, _ = Step(state, CompactionCompleted{At: testNow, CompactionID: "c-1"})
	if len(state.Deliveries) != 0 {
		t.Fatal("legacy completion invented a continuation")
	}
	state, _ = Step(state, SendRequested{At: testNow, Envelope: Envelope{ID: "legacy-random", From: Sender{Kind: SenderSelfDeclared, Name: "compact"}, To: "worker", Message: Message{Text: "continue"}, CreatedAt: testNow}})
	if len(state.Deliveries) != 1 || state.Deliveries["legacy-random"].Status != DeliveryDelivering {
		t.Fatalf("legacy completion/send replay changed: %v", state.Deliveries)
	}
}

func TestCompactionCompletionAndStopOrderingPreservesOneContinuation(t *testing.T) {
	for _, stopFirst := range []bool{false, true} {
		state := twoActiveHitches(t)
		state, _ = Step(state, CompactionRequested{At: testNow, Compaction: Compaction{ID: "c-1", HitchID: "worker-id", Resume: Message{Text: "continue"}, Deadline: testDeadline}})
		stop := TurnBoundaryReached{At: testNow, HitchID: "worker-id"}
		if stopFirst {
			state, _ = Step(state, stop)
		}
		state, _ = Step(state, completionWithContinuation(t))
		if !stopFirst {
			state, _ = Step(state, stop)
		}
		if len(state.Deliveries) != 1 || state.Deliveries["resume-c-1"].Status != DeliveryDelivering {
			t.Fatalf("stopFirst=%t lost in-flight continuation: %v", stopFirst, state.Deliveries)
		}
	}
}

func TestCompactionFastCompletionPrecedesSubmitOutcome(t *testing.T) {
	state := twoActiveHitches(t)
	state, _ = Step(state, CompactionRequested{At: testNow, Compaction: Compaction{ID: "c-1", HitchID: "worker-id", Resume: Message{Text: "continue"}, Deadline: testDeadline}})
	state, _ = Step(state, completionWithContinuation(t))
	state, effects := Step(state, CompactionSubmitted{At: testNow, CompactionID: "c-1"})
	if len(effects) != 0 || state.Compactions["c-1"].Status != CompactionSucceeded || len(state.Deliveries) != 1 {
		t.Fatalf("late submit outcome disturbed completion: %v %v", state.Compactions, effects)
	}
}
