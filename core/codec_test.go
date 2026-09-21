package core

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
	"time"
)

func TestEventCodecRoundTrip(t *testing.T) {
	now := time.Date(2026, 9, 21, 8, 0, 0, 0, time.UTC)
	deadline := now.Add(time.Minute)
	hitch := Hitch{ID: "h-1", Name: "worker", Collar: "codex", Directory: "/work"}
	envelope := Envelope{
		ID: "e-1", From: Sender{Kind: SenderAgent, Name: "lead", HitchID: "h-0"},
		To: "worker", Message: Message{Text: "hello"}, CreatedAt: now,
	}
	compact := Compaction{ID: "c-1", HitchID: "h-1", Resume: Message{Text: "continue"}, Deadline: deadline, Status: CompactionQueued}
	events := []Event{
		HitchRequested{At: now, Hitch: hitch, BootDeadline: deadline},
		AdoptRequested{At: now, Hitch: hitch, Pane: "%1"},
		RenameRequested{At: now, HitchID: "h-1", Name: "builder"},
		HitchSpawned{At: now, HitchID: "h-1", Pane: "%1"},
		HitchReady{At: now, HitchID: "h-1"},
		HitchLaunchFailed{At: now, HitchID: "h-1", Reason: "process exited"},
		TurnStarted{At: now, HitchID: "h-1"},
		TurnBoundaryReached{At: now, HitchID: "h-1"},
		SendRequested{At: now, Envelope: envelope, Deadline: deadline},
		SendRequested{At: now, Envelope: Envelope{ID: "e-2", From: Sender{Kind: SenderSelfDeclared, Name: "operator"}, To: "worker", Message: Message{Text: "later"}, CreatedAt: now}, Deadline: deadline, NotBefore: now.Add(30 * time.Second)},
		TimedDeliveryReleased{At: now, EnvelopeID: "e-1"},
		TimedDeliveriesCleared{At: now, Recipient: "worker"},
		DeliverySucceeded{At: now, EnvelopeID: "e-1"},
		DeliveryDeferred{At: now, EnvelopeID: "e-1", Reason: "busy"},
		DeliveryFailedEvent{At: now, EnvelopeID: "e-1", Reason: "pane gone"},
		DeliveryUnverifiedEvent{At: now, EnvelopeID: "e-1", Evidence: "composer changed"},
		CompactionRequested{At: now, Compaction: compact},
		CompactionCompleted{At: now, CompactionID: "c-1"},
		CompactionFailedEvent{At: now, CompactionID: "c-1", Reason: "command refused"},
		InterruptRequested{At: now, HitchID: "h-1", Reason: "stop", Deadline: deadline},
		InterruptSucceeded{At: now, HitchID: "h-1"},
		InterruptFailed{At: now, HitchID: "h-1", Reason: "key refused"},
		DropRequested{At: now, HitchID: "h-1", Deadline: deadline},
		DropSucceeded{At: now, HitchID: "h-1"},
		DropFailed{At: now, HitchID: "h-1", Reason: "pane busy"},
		WedgeDetected{At: now, HitchID: "h-1", Evidence: "unchanged for 5m"},
		WedgeCleared{At: now, HitchID: "h-1"},
		OperationTimedOut{At: now, Operation: TimeoutDelivery, ID: "e-1", Deadline: deadline, Evidence: "verification deadline passed"},
		CurfewSet{At: now, Deadline: deadline},
		CurfewCleared{At: now},
		TransitionRejected{At: now, Event: "drop_requested", Reason: "hitch is not active"},
	}

	for _, event := range events {
		t.Run(EventName(event), func(t *testing.T) {
			data, err := EncodeEvent(event)
			if err != nil {
				t.Fatal(err)
			}
			got, err := DecodeEvent(data)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, event) {
				t.Fatalf("round trip = %#v, want %#v", got, event)
			}
		})
	}
}

func TestEventSchemaRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name string
		data string
	}{
		{name: "unknown field", data: `{"type":"wedge_cleared","at":"2026-09-21T08:00:00Z","hitch_id":"h-1","surprise":true}`},
		{name: "invalid time", data: `{"type":"wedge_cleared","at":"later","hitch_id":"h-1"}`},
		{name: "empty evidence", data: `{"type":"wedge_detected","at":"2026-09-21T08:00:00Z","hitch_id":"h-1","evidence":""}`},
		{name: "unattributed sender", data: `{"type":"send_requested","at":"2026-09-21T08:00:00Z","deadline":"2026-09-21T08:01:00Z","envelope":{"id":"e-1","from":{"name":"lead"},"to":"worker","message":{"text":"hi"},"created_at":"2026-09-21T08:00:00Z"}}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := ValidateEventJSON([]byte(test.data)); err == nil {
				t.Fatal("invalid event passed validation")
			}
		})
	}
}

func TestPublishedJSONSchemaCoversEveryEvent(t *testing.T) {
	data, err := os.ReadFile("schema/events.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var schema map[string]any
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatalf("JSON Schema is not JSON: %v", err)
	}
	if schema["$schema"] != "https://json-schema.org/draft/2020-12/schema" {
		t.Fatalf("unexpected schema declaration: %#v", schema["$schema"])
	}
	definitions, ok := schema["$defs"].(map[string]any)
	if !ok {
		t.Fatal("JSON Schema has no definitions")
	}
	for _, name := range []string{
		"hitch_requested", "adopt_requested", "rename_requested", "hitch_spawned", "hitch_ready", "hitch_launch_failed",
		"turn_started", "turn_boundary_reached", "send_requested", "timed_delivery_released", "timed_deliveries_cleared",
		"delivery_succeeded", "delivery_deferred", "delivery_failed", "delivery_unverified",
		"compaction_requested", "compaction_completed", "compaction_failed",
		"interrupt_requested", "interrupt_succeeded", "interrupt_failed",
		"drop_requested", "drop_succeeded", "drop_failed", "wedge_detected", "wedge_cleared",
		"operation_timed_out", "curfew_set", "curfew_cleared", "transition_rejected",
	} {
		if _, exists := definitions[name]; !exists {
			t.Errorf("JSON Schema is missing %q", name)
		}
	}
}
