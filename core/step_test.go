package core

import (
	"reflect"
	"testing"
	"time"
)

func TestInputIntentRequiresMatchingResult(t *testing.T) {
	now := time.Date(2026, 9, 22, 0, 0, 0, 0, time.UTC)
	a := Agent{ID: "a", Status: Active, Activity: Idle}
	started, effects := Step(a, Event{Type: "input_started", At: now, HitchID: "a", ID: "m", Status: "envelope"})
	if started.Input == nil || started.Input.ID != "m" || len(effects) != 0 {
		t.Fatalf("intent not recorded: %+v %+v", started, effects)
	}
	wrong, effects := Step(started, Event{Type: "input_finished", At: now, HitchID: "a", ID: "other", Status: "delivered"})
	if !reflect.DeepEqual(wrong, started) || len(effects) != 1 {
		t.Fatal("unrelated result cleared input")
	}
	ended, effects := Step(started, Event{Type: "input_finished", At: now, HitchID: "a", ID: "m", Status: "unverified", Reason: "witness mismatch"})
	if ended.Input != nil || ended.Activity != Unknown || len(effects) != 0 {
		t.Fatalf("unverified result: %+v", ended)
	}
	if a.Input != nil || started.Input == nil {
		t.Fatal("Step mutated its input")
	}
}
func TestDeadlinesAreAgentLocalAndDoNotStopDrop(t *testing.T) {
	now := time.Date(2026, 9, 22, 0, 0, 0, 0, time.UTC)
	a := Agent{ID: "a", Status: Dropping, BootDeadline: now.Add(-time.Hour), DropDeadline: now.Add(-time.Hour), Activity: Interrupting, InterruptDeadline: now.Add(-time.Hour)}
	got, _ := Step(a, Event{Type: "deadline_checked", HitchID: "a", At: now})
	if !reflect.DeepEqual(a, got) {
		t.Fatal("deadline check changed a drop")
	}
	foreign, _ := Step(a, Event{Type: "hitch_ready", HitchID: "b", At: now})
	if !reflect.DeepEqual(a, foreign) {
		t.Fatal("another hitch changed this agent")
	}
	a.Status = Booting
	got, _ = Step(a, Event{Type: "deadline_checked", HitchID: "a", At: now})
	if got.Status != Failed {
		t.Fatalf("expired boot: %+v", got)
	}
}
func TestCompactionStepDoesNotMutateInput(t *testing.T) {
	now := time.Date(2026, 9, 22, 0, 0, 0, 0, time.UTC)
	a := Agent{ID: "a", Status: Active, Activity: Compacting, Compaction: &Compaction{ID: "c", Status: "submitted", Deadline: now.Add(-time.Second)}}
	got, _ := Step(a, Event{Type: "deadline_checked", HitchID: "a", At: now})
	if got.Compaction.Status != "unverified" || got.Activity != Unknown {
		t.Fatalf("expired compaction = %+v", got)
	}
	if a.Compaction.Status != "submitted" || a.Activity != Compacting {
		t.Fatalf("Step mutated input compaction: %+v", a)
	}
}
func TestEventValidationBeforeAppend(t *testing.T) {
	now := time.Date(2026, 9, 22, 0, 0, 0, 0, time.UTC)
	good := Event{Type: "native_hook", At: now, HitchID: "a", Name: "worker", Status: "activity"}
	data, err := EncodeEvent(good)
	if err != nil {
		t.Fatal(err)
	}
	got, err := DecodeEvent(data)
	if err != nil || !reflect.DeepEqual(got, good) {
		t.Fatalf("round trip: %+v %v", got, err)
	}
	for _, data := range []string{`{"type":"invented","at":"2026-09-22T00:00:00Z"}`, `{"type":"send_queued","at":"2026-09-22T00:00:00Z"}`, `{"type":"native_hook","at":"later"}`, `{"type":"native_hook","at":"2026-09-22T00:00:00Z","surprise":true}`} {
		if _, err := DecodeEvent([]byte(data)); err == nil {
			t.Fatalf("accepted invalid event %s", data)
		}
	}
}
