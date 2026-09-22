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

type eventRecord struct {
	Type         string           `json:"type"`
	At           time.Time        `json:"at"`
	Hitch        *Hitch           `json:"hitch,omitempty"`
	HitchID      HitchID          `json:"hitch_id,omitempty"`
	Name         AgentName        `json:"name,omitempty"`
	Pane         string           `json:"pane,omitempty"`
	BootDeadline *time.Time       `json:"boot_deadline,omitempty"`
	Envelope     *Envelope        `json:"envelope,omitempty"`
	EnvelopeID   EnvelopeID       `json:"envelope_id,omitempty"`
	Deadline     *time.Time       `json:"deadline,omitempty"`
	NotBefore    *time.Time       `json:"not_before,omitempty"`
	Recipient    AgentName        `json:"recipient,omitempty"`
	Compaction   *Compaction      `json:"compaction,omitempty"`
	CompactionID CompactionID     `json:"compaction_id,omitempty"`
	Operation    TimeoutOperation `json:"operation,omitempty"`
	ID           string           `json:"id,omitempty"`
	Event        string           `json:"event,omitempty"`
	Reason       string           `json:"reason,omitempty"`
	Evidence     string           `json:"evidence,omitempty"`
}

func EncodeEvent(event Event) ([]byte, error) {
	record := eventRecord{Type: EventName(event)}

	switch event := event.(type) {
	case HitchRequested:
		record.At, record.Hitch, record.BootDeadline = event.At, &event.Hitch, &event.BootDeadline
	case AdoptRequested:
		record.At, record.Hitch, record.Pane = event.At, &event.Hitch, event.Pane
	case RenameRequested:
		record.At, record.HitchID, record.Name = event.At, event.HitchID, event.Name
	case HitchSpawned:
		record.At, record.HitchID, record.Pane = event.At, event.HitchID, event.Pane
	case HitchReady:
		record.At, record.HitchID = event.At, event.HitchID
	case HitchLaunchFailed:
		record.At, record.HitchID, record.Reason = event.At, event.HitchID, event.Reason
	case TurnStarted:
		record.At, record.HitchID = event.At, event.HitchID
	case TurnBoundaryReached:
		record.At, record.HitchID = event.At, event.HitchID
	case BlockedDetected:
		record.At, record.HitchID, record.Evidence = event.At, event.HitchID, event.Evidence
	case BlockedCleared:
		record.At, record.HitchID = event.At, event.HitchID
	case SendRequested:
		record.At, record.Envelope, record.Deadline = event.At, &event.Envelope, &event.Deadline
		if !event.NotBefore.IsZero() {
			record.NotBefore = &event.NotBefore
		}
	case TimedDeliveryReleased:
		record.At, record.EnvelopeID = event.At, event.EnvelopeID
	case TimedDeliveriesCleared:
		record.At, record.Recipient = event.At, event.Recipient
	case DeliverySucceeded:
		record.At, record.EnvelopeID = event.At, event.EnvelopeID
	case DeliveryRetryRequested:
		record.At, record.EnvelopeID = event.At, event.EnvelopeID
	case DeliveryDeferred:
		record.At, record.EnvelopeID, record.Reason = event.At, event.EnvelopeID, event.Reason
	case DeliveryFailedEvent:
		record.At, record.EnvelopeID, record.Reason = event.At, event.EnvelopeID, event.Reason
	case DeliveryUnverifiedEvent:
		record.At, record.EnvelopeID, record.Evidence = event.At, event.EnvelopeID, event.Evidence
	case CompactionRequested:
		record.At, record.Compaction = event.At, &event.Compaction
	case CompactionCompleted:
		record.At, record.CompactionID = event.At, event.CompactionID
	case CompactionFailedEvent:
		record.At, record.CompactionID, record.Reason = event.At, event.CompactionID, event.Reason
	case InterruptRequested:
		record.At, record.HitchID, record.Reason, record.Deadline = event.At, event.HitchID, event.Reason, &event.Deadline
	case InterruptSucceeded:
		record.At, record.HitchID = event.At, event.HitchID
	case InterruptFailed:
		record.At, record.HitchID, record.Reason = event.At, event.HitchID, event.Reason
	case DropRequested:
		record.At, record.HitchID, record.Deadline = event.At, event.HitchID, &event.Deadline
	case DropSucceeded:
		record.At, record.HitchID = event.At, event.HitchID
	case DropFailed:
		record.At, record.HitchID, record.Reason = event.At, event.HitchID, event.Reason
	case PaneVanished:
		record.At, record.HitchID, record.Evidence = event.At, event.HitchID, event.Evidence
	case WedgeDetected:
		record.At, record.HitchID, record.Evidence = event.At, event.HitchID, event.Evidence
	case WedgeCleared:
		record.At, record.HitchID = event.At, event.HitchID
	case OperationTimedOut:
		record.At, record.Operation, record.ID = event.At, event.Operation, event.ID
		record.Deadline, record.Evidence = &event.Deadline, event.Evidence
	case CurfewSet:
		record.At, record.Deadline = event.At, &event.Deadline
	case CurfewCleared:
		record.At = event.At
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
		return HitchRequested{At: record.At, Hitch: *record.Hitch, BootDeadline: *record.BootDeadline}, nil
	case "adopt_requested":
		return AdoptRequested{At: record.At, Hitch: *record.Hitch, Pane: record.Pane}, nil
	case "rename_requested":
		return RenameRequested{At: record.At, HitchID: record.HitchID, Name: record.Name}, nil
	case "hitch_spawned":
		return HitchSpawned{At: record.At, HitchID: record.HitchID, Pane: record.Pane}, nil
	case "hitch_ready":
		return HitchReady{At: record.At, HitchID: record.HitchID}, nil
	case "hitch_launch_failed":
		return HitchLaunchFailed{At: record.At, HitchID: record.HitchID, Reason: record.Reason}, nil
	case "turn_started":
		return TurnStarted{At: record.At, HitchID: record.HitchID}, nil
	case "turn_boundary_reached":
		return TurnBoundaryReached{At: record.At, HitchID: record.HitchID}, nil
	case "blocked_detected":
		return BlockedDetected{At: record.At, HitchID: record.HitchID, Evidence: record.Evidence}, nil
	case "blocked_cleared":
		return BlockedCleared{At: record.At, HitchID: record.HitchID}, nil
	case "send_requested":
		event := SendRequested{At: record.At, Envelope: *record.Envelope, Deadline: *record.Deadline}
		if record.NotBefore != nil {
			event.NotBefore = *record.NotBefore
		}
		return event, nil
	case "timed_delivery_released":
		return TimedDeliveryReleased{At: record.At, EnvelopeID: record.EnvelopeID}, nil
	case "timed_deliveries_cleared":
		return TimedDeliveriesCleared{At: record.At, Recipient: record.Recipient}, nil
	case "delivery_succeeded":
		return DeliverySucceeded{At: record.At, EnvelopeID: record.EnvelopeID}, nil
	case "delivery_retry_requested":
		return DeliveryRetryRequested{At: record.At, EnvelopeID: record.EnvelopeID}, nil
	case "delivery_deferred":
		return DeliveryDeferred{At: record.At, EnvelopeID: record.EnvelopeID, Reason: record.Reason}, nil
	case "delivery_failed":
		return DeliveryFailedEvent{At: record.At, EnvelopeID: record.EnvelopeID, Reason: record.Reason}, nil
	case "delivery_unverified":
		return DeliveryUnverifiedEvent{At: record.At, EnvelopeID: record.EnvelopeID, Evidence: record.Evidence}, nil
	case "compaction_requested":
		return CompactionRequested{At: record.At, Compaction: *record.Compaction}, nil
	case "compaction_completed":
		return CompactionCompleted{At: record.At, CompactionID: record.CompactionID}, nil
	case "compaction_failed":
		return CompactionFailedEvent{At: record.At, CompactionID: record.CompactionID, Reason: record.Reason}, nil
	case "interrupt_requested":
		return InterruptRequested{At: record.At, HitchID: record.HitchID, Reason: record.Reason, Deadline: *record.Deadline}, nil
	case "interrupt_succeeded":
		return InterruptSucceeded{At: record.At, HitchID: record.HitchID}, nil
	case "interrupt_failed":
		return InterruptFailed{At: record.At, HitchID: record.HitchID, Reason: record.Reason}, nil
	case "drop_requested":
		return DropRequested{At: record.At, HitchID: record.HitchID, Deadline: *record.Deadline}, nil
	case "drop_succeeded":
		return DropSucceeded{At: record.At, HitchID: record.HitchID}, nil
	case "drop_failed":
		return DropFailed{At: record.At, HitchID: record.HitchID, Reason: record.Reason}, nil
	case "pane_vanished":
		return PaneVanished{At: record.At, HitchID: record.HitchID, Evidence: record.Evidence}, nil
	case "wedge_detected":
		return WedgeDetected{At: record.At, HitchID: record.HitchID, Evidence: record.Evidence}, nil
	case "wedge_cleared":
		return WedgeCleared{At: record.At, HitchID: record.HitchID}, nil
	case "operation_timed_out":
		return OperationTimedOut{At: record.At, Operation: record.Operation, ID: record.ID, Deadline: *record.Deadline, Evidence: record.Evidence}, nil
	case "curfew_set":
		return CurfewSet{At: record.At, Deadline: *record.Deadline}, nil
	case "curfew_cleared":
		return CurfewCleared{At: record.At}, nil
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
