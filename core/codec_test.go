package core

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

func TestEventCodecRoundTrip(t *testing.T) {
	now := time.Date(2026, 9, 21, 8, 0, 0, 0, time.UTC)
	events := []Event{
		HitchRequested{At: now, Hitch: Hitch{ID: "h-1", Name: "worker", Collar: "codex", Directory: "/work", Status: HitchStarting}},
		HitchReady{At: now, HitchID: "h-1", Pane: "%1"},
		HitchLaunchFailed{At: now, HitchID: "h-1", Reason: "process exited"},
		SendRequested{At: now, Envelope: Envelope{ID: "e-1", From: "lead", To: "worker", Message: Message{Text: "hello"}, CreatedAt: now}},
		DeliverySucceeded{At: now, EnvelopeID: "e-1"},
		DeliveryFailedEvent{At: now, EnvelopeID: "e-1", Reason: "pane gone"},
		DropRequested{At: now, HitchID: "h-1"},
		DropSucceeded{At: now, HitchID: "h-1"},
		DropFailed{At: now, HitchID: "h-1", Reason: "pane busy"},
		TransitionRejected{At: now, Event: "drop_requested", Reason: "hitch is not active"},
	}

	for _, event := range events {
		t.Run(eventName(event), func(t *testing.T) {
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

func TestEventSchemaRejectsUnknownFields(t *testing.T) {
	data := []byte(`{"type":"drop_requested","at":"2026-09-21T08:00:00Z","hitch_id":"h-1","surprise":true}`)
	if err := ValidateEventJSON(data); err == nil {
		t.Fatal("unknown field passed event validation")
	}
}

func TestEventSchemaRejectsInvalidTime(t *testing.T) {
	data := []byte(`{"type":"drop_requested","at":"later","hitch_id":"h-1"}`)
	if err := ValidateEventJSON(data); err == nil {
		t.Fatal("invalid timestamp passed event validation")
	}
}

func TestEventJSONSchemaIsExported(t *testing.T) {
	var schema map[string]any
	if err := json.Unmarshal(EventJSONSchema(), &schema); err != nil {
		t.Fatalf("JSON Schema is not JSON: %v", err)
	}
	if schema["$schema"] != "https://json-schema.org/draft/2020-12/schema" {
		t.Fatalf("unexpected schema declaration: %#v", schema["$schema"])
	}
}
