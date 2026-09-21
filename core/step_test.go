package core

import (
	"reflect"
	"testing"
	"time"
)

func TestStepLifecycle(t *testing.T) {
	now := time.Date(2026, 9, 21, 8, 0, 0, 0, time.UTC)
	hitch := Hitch{
		ID:        "h-1",
		Name:      "worker",
		Collar:    "codex",
		Directory: "/work",
	}
	envelope := Envelope{
		ID:        "e-1",
		From:      "lead",
		To:        "worker",
		Message:   Message{Text: "build it"},
		CreatedAt: now,
	}

	tests := []struct {
		name       string
		event      Event
		wantStatus HitchStatus
		wantEffect Effect
	}{
		{
			name:       "hitch requests a spawn",
			event:      HitchRequested{At: now, Hitch: hitch},
			wantStatus: HitchStarting,
			wantEffect: SpawnHitch{Hitch: Hitch{ID: "h-1", Name: "worker", Collar: "codex", Directory: "/work", Status: HitchStarting}},
		},
		{
			name:       "ready activates hitch",
			event:      HitchReady{At: now, HitchID: "h-1", Pane: "%1"},
			wantStatus: HitchActive,
		},
		{
			name:       "send requests delivery",
			event:      SendRequested{At: now, Envelope: envelope},
			wantStatus: HitchActive,
			wantEffect: DeliverEnvelope{Envelope: envelope, Pane: "%1"},
		},
		{
			name:       "delivery outcome updates state",
			event:      DeliverySucceeded{At: now, EnvelopeID: "e-1"},
			wantStatus: HitchActive,
		},
		{
			name:       "drop requests pane kill",
			event:      DropRequested{At: now, HitchID: "h-1"},
			wantStatus: HitchDropping,
			wantEffect: KillHitch{HitchID: "h-1", Pane: "%1"},
		},
		{
			name:       "drop outcome closes hitch",
			event:      DropSucceeded{At: now, HitchID: "h-1"},
			wantStatus: HitchDropped,
		},
	}

	state := NewState(Team{ID: "team-1", Name: "example"})
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var effects []Effect
			state, effects = Step(state, test.event)
			if got := state.Hitches["h-1"].Status; got != test.wantStatus {
				t.Fatalf("status = %q, want %q", got, test.wantStatus)
			}
			if test.wantEffect == nil {
				if len(effects) != 0 {
					t.Fatalf("effects = %#v, want none", effects)
				}
				return
			}
			if len(effects) != 1 || !reflect.DeepEqual(effects[0], test.wantEffect) {
				t.Fatalf("effects = %#v, want %#v", effects, test.wantEffect)
			}
		})
	}

	if got := state.Deliveries["e-1"].Status; got != DeliveryDelivered {
		t.Fatalf("delivery status = %q, want %q", got, DeliveryDelivered)
	}
}

func TestStepRejectsInvalidTransitionWithoutMutatingInput(t *testing.T) {
	now := time.Date(2026, 9, 21, 8, 0, 0, 0, time.UTC)
	state := NewState(Team{ID: "team-1", Name: "example"})

	next, effects := Step(state, DropRequested{At: now, HitchID: "missing"})
	if len(state.Hitches) != 0 || len(next.Hitches) != 0 {
		t.Fatalf("invalid transition mutated state: before=%#v after=%#v", state, next)
	}
	if len(effects) != 1 {
		t.Fatalf("effects = %#v, want one rejection", effects)
	}
	record, ok := effects[0].(RecordEvent)
	if !ok {
		t.Fatalf("effect = %T, want RecordEvent", effects[0])
	}
	rejection, ok := record.Event.(TransitionRejected)
	if !ok || rejection.Event != "drop_requested" || rejection.Reason == "" {
		t.Fatalf("rejection = %#v", record.Event)
	}
}
