package core

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"time"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

//go:embed schema/events.cue
var eventSchema []byte

//go:embed schema/events.schema.json
var eventJSONSchema []byte

type eventRecord struct {
	Type       string     `json:"type"`
	At         time.Time  `json:"at"`
	Hitch      *Hitch     `json:"hitch,omitempty"`
	HitchID    HitchID    `json:"hitch_id,omitempty"`
	Pane       string     `json:"pane,omitempty"`
	Envelope   *Envelope  `json:"envelope,omitempty"`
	EnvelopeID EnvelopeID `json:"envelope_id,omitempty"`
	Event      string     `json:"event,omitempty"`
	Reason     string     `json:"reason,omitempty"`
}

func EncodeEvent(event Event) ([]byte, error) {
	record := eventRecord{Type: eventName(event)}

	switch event := event.(type) {
	case HitchRequested:
		record.At, record.Hitch = event.At, &event.Hitch
	case HitchReady:
		record.At, record.HitchID, record.Pane = event.At, event.HitchID, event.Pane
	case HitchLaunchFailed:
		record.At, record.HitchID, record.Reason = event.At, event.HitchID, event.Reason
	case SendRequested:
		record.At, record.Envelope = event.At, &event.Envelope
	case DeliverySucceeded:
		record.At, record.EnvelopeID = event.At, event.EnvelopeID
	case DeliveryFailedEvent:
		record.At, record.EnvelopeID, record.Reason = event.At, event.EnvelopeID, event.Reason
	case DropRequested:
		record.At, record.HitchID = event.At, event.HitchID
	case DropSucceeded:
		record.At, record.HitchID = event.At, event.HitchID
	case DropFailed:
		record.At, record.HitchID, record.Reason = event.At, event.HitchID, event.Reason
	case TransitionRejected:
		record.At, record.Event, record.Reason = event.At, event.Event, event.Reason
	}

	data, err := json.Marshal(record)
	if err != nil {
		return nil, fmt.Errorf("encode event: %w", err)
	}
	if err := ValidateEventJSON(data); err != nil {
		return nil, err
	}
	return data, nil
}

func DecodeEvent(data []byte) (Event, error) {
	if err := ValidateEventJSON(data); err != nil {
		return nil, err
	}

	var record eventRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return nil, fmt.Errorf("decode event: %w", err)
	}

	switch record.Type {
	case "hitch_requested":
		return HitchRequested{At: record.At, Hitch: *record.Hitch}, nil
	case "hitch_ready":
		return HitchReady{At: record.At, HitchID: record.HitchID, Pane: record.Pane}, nil
	case "hitch_launch_failed":
		return HitchLaunchFailed{At: record.At, HitchID: record.HitchID, Reason: record.Reason}, nil
	case "send_requested":
		return SendRequested{At: record.At, Envelope: *record.Envelope}, nil
	case "delivery_succeeded":
		return DeliverySucceeded{At: record.At, EnvelopeID: record.EnvelopeID}, nil
	case "delivery_failed":
		return DeliveryFailedEvent{At: record.At, EnvelopeID: record.EnvelopeID, Reason: record.Reason}, nil
	case "drop_requested":
		return DropRequested{At: record.At, HitchID: record.HitchID}, nil
	case "drop_succeeded":
		return DropSucceeded{At: record.At, HitchID: record.HitchID}, nil
	case "drop_failed":
		return DropFailed{At: record.At, HitchID: record.HitchID, Reason: record.Reason}, nil
	case "transition_rejected":
		return TransitionRejected{At: record.At, Event: record.Event, Reason: record.Reason}, nil
	default:
		return nil, fmt.Errorf("decode event: unsupported type %q", record.Type)
	}
}

func ValidateEventJSON(data []byte) error {
	ctx := cuecontext.New()
	schema := ctx.CompileBytes(eventSchema, cue.Filename("events.cue")).LookupPath(cue.MakePath(cue.Def("Event")))
	if err := schema.Err(); err != nil {
		return fmt.Errorf("compile event schema: %w", err)
	}
	value := ctx.CompileBytes(data, cue.Filename("event.json"))
	if err := value.Err(); err != nil {
		return fmt.Errorf("parse event: %w", err)
	}
	if err := schema.Unify(value).Validate(cue.Concrete(true)); err != nil {
		return fmt.Errorf("validate event: %w", err)
	}
	return nil
}

func EventJSONSchema() []byte {
	return append([]byte(nil), eventJSONSchema...)
}
