package core

import (
	"fmt"
	"sort"
	"time"
)

// Step applies one durable fact. It performs no I/O and reads no ambient time.
func Step(state State, event Event) (State, []Effect) {
	next := cloneState(state)

	switch event := event.(type) {
	case HitchRequested:
		return stepHitchRequested(next, event)
	case AdoptRequested:
		return stepAdoptRequested(next, event)
	case RenameRequested:
		return stepRenameRequested(next, event)
	case HitchSpawned:
		return stepHitchSpawned(next, event)
	case HitchReady:
		return stepHitchReady(next, event)
	case HitchLaunchFailed:
		return stepHitchLaunchFailed(next, event)
	case TurnStarted:
		return stepTurnStarted(next, event)
	case TurnBoundaryReached:
		return stepTurnBoundary(next, event)
	case SendRequested:
		return stepSendRequested(next, event)
	case TimedDeliveryReleased:
		return stepTimedDeliveryReleased(next, event)
	case TimedDeliveriesCleared:
		return stepTimedDeliveriesCleared(next, event)
	case DeliverySucceeded:
		return stepDeliverySucceeded(next, event)
	case DeliveryDeferred:
		return stepDeliveryDeferred(next, event)
	case DeliveryFailedEvent:
		return stepDeliveryFailed(next, event)
	case DeliveryUnverifiedEvent:
		return stepDeliveryUnverified(next, event)
	case CompactionRequested:
		return stepCompactionRequested(next, event)
	case CompactionCompleted:
		return stepCompactionCompleted(next, event)
	case CompactionFailedEvent:
		return stepCompactionFailed(next, event)
	case InterruptRequested:
		return stepInterruptRequested(next, event)
	case InterruptSucceeded:
		return stepInterruptSucceeded(next, event)
	case InterruptFailed:
		return stepInterruptFailed(next, event)
	case DropRequested:
		return stepDropRequested(next, event)
	case DropSucceeded:
		return stepDropSucceeded(next, event)
	case DropFailed:
		return stepDropFailed(next, event)
	case WedgeDetected:
		return stepWedgeDetected(next, event)
	case WedgeCleared:
		return stepWedgeCleared(next, event)
	case OperationTimedOut:
		return stepTimedOut(next, event)
	case CurfewSet:
		return stepCurfewSet(next, event)
	case CurfewCleared:
		return stepCurfewCleared(next, event)
	case TransitionRejected:
		return next, nil
	}

	return next, nil
}

func stepAdoptRequested(state State, event AdoptRequested) (State, []Effect) {
	if _, exists := state.Hitches[event.Hitch.ID]; exists {
		return rejected(state, event, event.At, "hitch id already exists")
	}
	if event.Hitch.ID == "" || event.Hitch.Name == "" || event.Hitch.Collar == "" || event.Hitch.Directory == "" || event.Pane == "" {
		return rejected(state, event, event.At, "hitch id, name, collar, directory, and pane are required")
	}
	for _, hitch := range state.Hitches {
		if hitch.Name == event.Hitch.Name && hitchOccupiesName(hitch.Status) {
			return rejected(state, event, event.At, "hitch name already exists")
		}
	}
	hitch := event.Hitch
	hitch.Status = HitchActive
	hitch.Activity = ActivityIdle
	hitch.Pane = event.Pane
	hitch.BootDeadline = time.Time{}
	hitch.DropDeadline = time.Time{}
	hitch.InterruptDeadline = time.Time{}
	hitch.InterruptReason = ""
	hitch.PendingCompactID = ""
	hitch.WedgeEvidence = ""
	hitch.PreviousActivity = ""
	state.Hitches[hitch.ID] = hitch
	return state, nil
}

func stepRenameRequested(state State, event RenameRequested) (State, []Effect) {
	hitch, ok := activeHitchByID(state, event.HitchID)
	if !ok || event.Name == "" {
		return rejected(state, event, event.At, "hitch is not active or name is empty")
	}
	for _, other := range state.Hitches {
		if other.ID != hitch.ID && other.Name == event.Name && hitchOccupiesName(other.Status) {
			return rejected(state, event, event.At, "hitch name already exists")
		}
	}
	oldName := hitch.Name
	hitch.Name = event.Name
	state.Hitches[hitch.ID] = hitch
	for id, delivery := range state.Deliveries {
		if delivery.Envelope.To == oldName && (delivery.Status == DeliveryQueued || delivery.Status == DeliveryDelivering) {
			delivery.Envelope.To = event.Name
			state.Deliveries[id] = delivery
		}
	}
	return state, nil
}

func stepHitchRequested(state State, event HitchRequested) (State, []Effect) {
	if _, exists := state.Hitches[event.Hitch.ID]; exists {
		return rejected(state, event, event.At, "hitch id already exists")
	}
	if event.Hitch.ID == "" || event.Hitch.Name == "" || event.Hitch.Collar == "" || event.Hitch.Directory == "" {
		return rejected(state, event, event.At, "hitch id, name, collar, and directory are required")
	}
	if !validDeadline(event.At, event.BootDeadline) {
		return rejected(state, event, event.At, "boot deadline must be after event time")
	}
	for _, hitch := range state.Hitches {
		if hitch.Name == event.Hitch.Name && hitchOccupiesName(hitch.Status) {
			return rejected(state, event, event.At, "hitch name already exists")
		}
	}
	hitch := event.Hitch
	hitch.Status = HitchStarting
	hitch.Activity = ActivityUnknown
	hitch.Pane = ""
	hitch.BootDeadline = event.BootDeadline
	hitch.DropDeadline = time.Time{}
	hitch.PendingCompactID = ""
	hitch.WedgeEvidence = ""
	hitch.PreviousActivity = ""
	state.Hitches[hitch.ID] = hitch
	return state, []Effect{SpawnHitch{Hitch: hitch}}
}

func stepHitchSpawned(state State, event HitchSpawned) (State, []Effect) {
	hitch, ok := state.Hitches[event.HitchID]
	if !ok || hitch.Status != HitchStarting || event.Pane == "" {
		return rejected(state, event, event.At, "hitch is not starting or pane is empty")
	}
	hitch.Status = HitchBooting
	hitch.Pane = event.Pane
	state.Hitches[hitch.ID] = hitch
	return state, []Effect{AwaitBoot{HitchID: hitch.ID, Pane: hitch.Pane, Deadline: hitch.BootDeadline}}
}

func stepHitchReady(state State, event HitchReady) (State, []Effect) {
	hitch, ok := state.Hitches[event.HitchID]
	if !ok || hitch.Status != HitchBooting {
		return rejected(state, event, event.At, "hitch is not booting")
	}
	hitch.Status = HitchActive
	hitch.Activity = ActivityIdle
	hitch.BootDeadline = time.Time{}
	state.Hitches[hitch.ID] = hitch
	return dispatchNext(state, hitch.ID)
}

func stepHitchLaunchFailed(state State, event HitchLaunchFailed) (State, []Effect) {
	hitch, ok := state.Hitches[event.HitchID]
	if !ok || (hitch.Status != HitchStarting && hitch.Status != HitchBooting) || event.Reason == "" {
		return rejected(state, event, event.At, "hitch is not starting or booting, or reason is empty")
	}
	hitch.Status = HitchFailed
	hitch.Activity = ActivityUnknown
	hitch.BootDeadline = time.Time{}
	state.Hitches[hitch.ID] = hitch
	return state, nil
}

func stepTurnStarted(state State, event TurnStarted) (State, []Effect) {
	hitch, ok := activeHitchByID(state, event.HitchID)
	if !ok || hitch.Activity != ActivityIdle {
		return rejected(state, event, event.At, "hitch is not active and idle")
	}
	hitch.Activity = ActivityBusy
	state.Hitches[hitch.ID] = hitch
	return state, nil
}

func stepTurnBoundary(state State, event TurnBoundaryReached) (State, []Effect) {
	hitch, ok := activeHitchByID(state, event.HitchID)
	if !ok || (hitch.Activity != ActivityBusy && hitch.Activity != ActivityWedged) {
		return rejected(state, event, event.At, "hitch is not in a turn or wedged")
	}
	hitch.Activity = ActivityIdle
	hitch.WedgeEvidence = ""
	state.Hitches[hitch.ID] = hitch
	return dispatchNext(state, hitch.ID)
}

func stepSendRequested(state State, event SendRequested) (State, []Effect) {
	envelope := event.Envelope
	if _, exists := state.Deliveries[envelope.ID]; exists {
		return rejected(state, event, event.At, "envelope id already exists")
	}
	if envelope.ID == "" || envelope.To == "" || envelope.Message.Text == "" {
		return rejected(state, event, event.At, "envelope id, recipient, and message are required")
	}
	if !validDeadline(event.At, event.Deadline) {
		return rejected(state, event, event.At, "delivery deadline must be after event time")
	}
	if !event.NotBefore.IsZero() && !event.NotBefore.After(event.At) {
		return rejected(state, event, event.At, "not-before time must be after event time")
	}
	if reason := invalidSender(state, envelope.From); reason != "" {
		return rejected(state, event, event.At, reason)
	}
	hitch, ok := activeHitchByName(state, envelope.To)
	if !ok {
		return rejected(state, event, event.At, "recipient is not active")
	}
	if envelope.From.Kind == SenderAgent && envelope.From.HitchID == hitch.ID {
		return rejected(state, event, event.At, "sender and recipient are the same hitch")
	}
	state.Deliveries[envelope.ID] = Delivery{
		Envelope:  envelope,
		Status:    DeliveryQueued,
		Deadline:  event.Deadline,
		NotBefore: event.NotBefore,
	}
	state.DeliveryOrder = append(state.DeliveryOrder, envelope.ID)
	return dispatchNext(state, hitch.ID)
}

func stepTimedDeliveryReleased(state State, event TimedDeliveryReleased) (State, []Effect) {
	delivery, ok := state.Deliveries[event.EnvelopeID]
	if !ok || delivery.Status != DeliveryQueued || delivery.NotBefore.IsZero() {
		return rejected(state, event, event.At, "delivery is not waiting for its scheduled time")
	}
	if event.At.Before(delivery.NotBefore) {
		return rejected(state, event, event.At, "delivery is not due yet")
	}
	delivery.NotBefore = time.Time{}
	state.Deliveries[event.EnvelopeID] = delivery
	hitch, ok := activeHitchByName(state, delivery.Envelope.To)
	if !ok {
		return state, nil
	}
	return dispatchNext(state, hitch.ID)
}

func stepTimedDeliveriesCleared(state State, event TimedDeliveriesCleared) (State, []Effect) {
	if event.Recipient == "" {
		return rejected(state, event, event.At, "recipient is required")
	}
	for id, delivery := range state.Deliveries {
		if delivery.Envelope.To != event.Recipient || delivery.Status != DeliveryQueued || delivery.NotBefore.IsZero() {
			continue
		}
		delivery.Status = DeliveryCancelled
		delivery.Reason = "scheduled delivery cleared"
		state.Deliveries[id] = delivery
	}
	return state, nil
}

func stepDeliverySucceeded(state State, event DeliverySucceeded) (State, []Effect) {
	delivery, hitch, ok := activeDelivery(state, event.EnvelopeID)
	if !ok {
		return rejected(state, event, event.At, "delivery is not in progress")
	}
	delivery.Status = DeliveryDelivered
	state.Deliveries[event.EnvelopeID] = delivery
	hitch.Activity = ActivityBusy
	state.Hitches[hitch.ID] = hitch
	return state, nil
}

func stepDeliveryDeferred(state State, event DeliveryDeferred) (State, []Effect) {
	delivery, hitch, ok := activeDelivery(state, event.EnvelopeID)
	if !ok || event.Reason == "" {
		return rejected(state, event, event.At, "delivery is not in progress or reason is empty")
	}
	delivery.Status = DeliveryQueued
	delivery.Reason = event.Reason
	state.Deliveries[event.EnvelopeID] = delivery
	hitch.Activity = ActivityBusy
	state.Hitches[hitch.ID] = hitch
	return state, nil
}

func stepDeliveryFailed(state State, event DeliveryFailedEvent) (State, []Effect) {
	delivery, hitch, ok := activeDelivery(state, event.EnvelopeID)
	if !ok || event.Reason == "" {
		return rejected(state, event, event.At, "delivery is not in progress or reason is empty")
	}
	delivery.Status = DeliveryFailed
	delivery.Reason = event.Reason
	state.Deliveries[event.EnvelopeID] = delivery
	hitch.Activity = ActivityIdle
	state.Hitches[hitch.ID] = hitch
	return dispatchNext(state, hitch.ID)
}

func stepDeliveryUnverified(state State, event DeliveryUnverifiedEvent) (State, []Effect) {
	delivery, hitch, ok := activeDelivery(state, event.EnvelopeID)
	if !ok || event.Evidence == "" {
		return rejected(state, event, event.At, "delivery is not in progress or evidence is empty")
	}
	delivery.Status = DeliveryUnverified
	delivery.Reason = event.Evidence
	state.Deliveries[event.EnvelopeID] = delivery
	hitch.Activity = ActivityWedged
	hitch.WedgeEvidence = event.Evidence
	state.Hitches[hitch.ID] = hitch
	return state, nil
}

func stepCompactionRequested(state State, event CompactionRequested) (State, []Effect) {
	compact := event.Compaction
	if _, exists := state.Compactions[compact.ID]; exists {
		return rejected(state, event, event.At, "compaction id already exists")
	}
	hitch, ok := activeHitchByID(state, compact.HitchID)
	if !ok || compact.ID == "" || compact.Resume.Text == "" {
		return rejected(state, event, event.At, "active hitch, compaction id, and resume message are required")
	}
	if hitch.PendingCompactID != "" {
		return rejected(state, event, event.At, "hitch already has a pending compaction")
	}
	if !validDeadline(event.At, compact.Deadline) {
		return rejected(state, event, event.At, "compaction deadline must be after event time")
	}
	compact.Status = CompactionQueued
	compact.Reason = ""
	state.Compactions[compact.ID] = compact
	hitch.PendingCompactID = compact.ID
	state.Hitches[hitch.ID] = hitch
	return dispatchNext(state, hitch.ID)
}

func stepCompactionCompleted(state State, event CompactionCompleted) (State, []Effect) {
	compact, hitch, ok := activeCompaction(state, event.CompactionID)
	if !ok {
		return rejected(state, event, event.At, "compaction is not in progress")
	}
	compact.Status = CompactionSucceeded
	state.Compactions[compact.ID] = compact
	hitch.PendingCompactID = ""
	hitch.Activity = ActivityBusy
	state.Hitches[hitch.ID] = hitch
	return state, nil
}

func stepCompactionFailed(state State, event CompactionFailedEvent) (State, []Effect) {
	compact, hitch, ok := activeCompaction(state, event.CompactionID)
	if !ok || event.Reason == "" {
		return rejected(state, event, event.At, "compaction is not in progress or reason is empty")
	}
	compact.Status = CompactionFailed
	compact.Reason = event.Reason
	state.Compactions[compact.ID] = compact
	hitch.PendingCompactID = ""
	hitch.Activity = ActivityIdle
	state.Hitches[hitch.ID] = hitch
	return dispatchNext(state, hitch.ID)
}

func stepInterruptRequested(state State, event InterruptRequested) (State, []Effect) {
	hitch, ok := activeHitchByID(state, event.HitchID)
	if !ok || (hitch.Activity != ActivityBusy && hitch.Activity != ActivityWedged) {
		return rejected(state, event, event.At, "hitch is not in an interruptible turn")
	}
	if !validDeadline(event.At, event.Deadline) {
		return rejected(state, event, event.At, "interrupt deadline must be after event time")
	}
	hitch.PreviousActivity = hitch.Activity
	hitch.Activity = ActivityInterrupting
	hitch.InterruptDeadline = event.Deadline
	hitch.InterruptReason = event.Reason
	state.Hitches[hitch.ID] = hitch
	return state, []Effect{InterruptHitch{HitchID: hitch.ID, Pane: hitch.Pane, Reason: event.Reason, Deadline: event.Deadline}}
}

func stepInterruptSucceeded(state State, event InterruptSucceeded) (State, []Effect) {
	hitch, ok := activeHitchByID(state, event.HitchID)
	if !ok || hitch.Activity != ActivityInterrupting {
		return rejected(state, event, event.At, "interrupt is not in progress")
	}
	hitch.Activity = ActivityBusy
	hitch.InterruptDeadline = time.Time{}
	hitch.InterruptReason = ""
	hitch.PreviousActivity = ""
	state.Hitches[hitch.ID] = hitch
	return state, nil
}

func stepInterruptFailed(state State, event InterruptFailed) (State, []Effect) {
	hitch, ok := activeHitchByID(state, event.HitchID)
	if !ok || hitch.Activity != ActivityInterrupting || event.Reason == "" {
		return rejected(state, event, event.At, "interrupt is not in progress or reason is empty")
	}
	hitch.Activity = hitch.PreviousActivity
	hitch.InterruptDeadline = time.Time{}
	hitch.InterruptReason = ""
	hitch.PreviousActivity = ""
	state.Hitches[hitch.ID] = hitch
	return state, nil
}

func stepDropRequested(state State, event DropRequested) (State, []Effect) {
	hitch, ok := activeHitchByID(state, event.HitchID)
	if !ok {
		return rejected(state, event, event.At, "hitch is not active")
	}
	if !validDeadline(event.At, event.Deadline) {
		return rejected(state, event, event.At, "drop deadline must be after event time")
	}
	hitch.PreviousActivity = hitch.Activity
	hitch.Status = HitchDropping
	hitch.Activity = ActivityUnknown
	hitch.DropDeadline = event.Deadline
	hitch.InterruptDeadline = time.Time{}
	hitch.InterruptReason = ""
	if hitch.PendingCompactID != "" {
		compact := state.Compactions[hitch.PendingCompactID]
		compact.Status = CompactionCancelled
		compact.Reason = "hitch is dropping"
		state.Compactions[compact.ID] = compact
		hitch.PendingCompactID = ""
	}
	for id, delivery := range state.Deliveries {
		if delivery.Envelope.To != hitch.Name || (delivery.Status != DeliveryQueued && delivery.Status != DeliveryDelivering) {
			continue
		}
		delivery.Status = DeliveryCancelled
		delivery.Reason = "recipient is dropping"
		state.Deliveries[id] = delivery
	}
	state.Hitches[hitch.ID] = hitch
	return state, []Effect{KillHitch{HitchID: hitch.ID, Pane: hitch.Pane, Deadline: event.Deadline}}
}

func stepDropSucceeded(state State, event DropSucceeded) (State, []Effect) {
	hitch, ok := state.Hitches[event.HitchID]
	if !ok || hitch.Status != HitchDropping {
		return rejected(state, event, event.At, "hitch is not dropping")
	}
	hitch.Status = HitchDropped
	hitch.Activity = ActivityUnknown
	hitch.Pane = ""
	hitch.DropDeadline = time.Time{}
	hitch.PreviousActivity = ""
	state.Hitches[hitch.ID] = hitch
	return state, nil
}

func stepDropFailed(state State, event DropFailed) (State, []Effect) {
	hitch, ok := state.Hitches[event.HitchID]
	if !ok || hitch.Status != HitchDropping || event.Reason == "" {
		return rejected(state, event, event.At, "hitch is not dropping or reason is empty")
	}
	hitch.Status = HitchActive
	hitch.Activity = hitch.PreviousActivity
	if hitch.Activity == ActivityDelivering || hitch.Activity == ActivityCompacting || hitch.Activity == ActivityInterrupting {
		hitch.Activity = ActivityWedged
		hitch.WedgeEvidence = event.Reason
	}
	hitch.DropDeadline = time.Time{}
	hitch.PreviousActivity = ""
	state.Hitches[hitch.ID] = hitch
	return state, nil
}

func stepWedgeDetected(state State, event WedgeDetected) (State, []Effect) {
	hitch, ok := activeHitchByID(state, event.HitchID)
	if !ok || hitch.Activity != ActivityBusy || event.Evidence == "" {
		return rejected(state, event, event.At, "hitch is not in a turn or evidence is empty")
	}
	hitch.Activity = ActivityWedged
	hitch.WedgeEvidence = event.Evidence
	state.Hitches[hitch.ID] = hitch
	return state, nil
}

func stepWedgeCleared(state State, event WedgeCleared) (State, []Effect) {
	hitch, ok := activeHitchByID(state, event.HitchID)
	if !ok || hitch.Activity != ActivityWedged {
		return rejected(state, event, event.At, "hitch is not wedged")
	}
	hitch.Activity = ActivityIdle
	hitch.WedgeEvidence = ""
	state.Hitches[hitch.ID] = hitch
	return dispatchNext(state, hitch.ID)
}

func stepTimedOut(state State, event OperationTimedOut) (State, []Effect) {
	if event.Evidence == "" {
		return rejected(state, event, event.At, "timeout evidence is empty")
	}
	switch event.Operation {
	case TimeoutBoot:
		hitch, ok := state.Hitches[HitchID(event.ID)]
		if !ok || (hitch.Status != HitchStarting && hitch.Status != HitchBooting) || !event.Deadline.Equal(hitch.BootDeadline) {
			return rejected(state, event, event.At, "boot timeout does not match a pending boot")
		}
		hitch.Status = HitchFailed
		hitch.Activity = ActivityUnknown
		hitch.BootDeadline = time.Time{}
		hitch.WedgeEvidence = event.Evidence
		state.Hitches[hitch.ID] = hitch
		return state, nil
	case TimeoutTurn:
		hitch, ok := activeHitchByID(state, HitchID(event.ID))
		if !ok || hitch.Activity != ActivityBusy {
			return rejected(state, event, event.At, "turn timeout does not match a busy hitch")
		}
		hitch.Activity = ActivityWedged
		hitch.WedgeEvidence = event.Evidence
		state.Hitches[hitch.ID] = hitch
		return state, nil
	case TimeoutDelivery:
		delivery, hitch, ok := activeDelivery(state, EnvelopeID(event.ID))
		if !ok || !event.Deadline.Equal(delivery.Deadline) {
			return rejected(state, event, event.At, "delivery timeout does not match an in-progress delivery")
		}
		delivery.Status = DeliveryUnverified
		delivery.Reason = event.Evidence
		state.Deliveries[delivery.Envelope.ID] = delivery
		hitch.Activity = ActivityWedged
		hitch.WedgeEvidence = event.Evidence
		state.Hitches[hitch.ID] = hitch
		return state, nil
	case TimeoutCompaction:
		compact, hitch, ok := activeCompaction(state, CompactionID(event.ID))
		if !ok || !event.Deadline.Equal(compact.Deadline) {
			return rejected(state, event, event.At, "compaction timeout does not match an in-progress compaction")
		}
		compact.Status = CompactionUnverified
		compact.Reason = event.Evidence
		state.Compactions[compact.ID] = compact
		hitch.PendingCompactID = ""
		hitch.Activity = ActivityWedged
		hitch.WedgeEvidence = event.Evidence
		state.Hitches[hitch.ID] = hitch
		return state, nil
	case TimeoutInterrupt:
		hitch, ok := activeHitchByID(state, HitchID(event.ID))
		if !ok || hitch.Activity != ActivityInterrupting || !event.Deadline.Equal(hitch.InterruptDeadline) {
			return rejected(state, event, event.At, "interrupt timeout does not match an in-progress interrupt")
		}
		hitch.Activity = ActivityWedged
		hitch.InterruptDeadline = time.Time{}
		hitch.InterruptReason = ""
		hitch.PreviousActivity = ""
		hitch.WedgeEvidence = event.Evidence
		state.Hitches[hitch.ID] = hitch
		return state, nil
	case TimeoutDrop:
		hitch, ok := state.Hitches[HitchID(event.ID)]
		if !ok || hitch.Status != HitchDropping || !event.Deadline.Equal(hitch.DropDeadline) {
			return rejected(state, event, event.At, "drop timeout does not match a pending drop")
		}
		hitch.Status = HitchActive
		hitch.Activity = ActivityWedged
		hitch.DropDeadline = time.Time{}
		hitch.PreviousActivity = ""
		hitch.WedgeEvidence = event.Evidence
		state.Hitches[hitch.ID] = hitch
		return state, nil
	default:
		return rejected(state, event, event.At, "unknown timeout operation")
	}
}

func stepCurfewSet(state State, event CurfewSet) (State, []Effect) {
	if !validDeadline(event.At, event.Deadline) {
		return rejected(state, event, event.At, "curfew deadline must be after event time")
	}
	state.Team.Curfew = event.Deadline
	return state, nil
}

func stepCurfewCleared(state State, event CurfewCleared) (State, []Effect) {
	if state.Team.Curfew.IsZero() {
		return rejected(state, event, event.At, "team has no curfew")
	}
	state.Team.Curfew = time.Time{}
	return state, nil
}

func dispatchNext(state State, hitchID HitchID) (State, []Effect) {
	hitch, ok := activeHitchByID(state, hitchID)
	if !ok || hitch.Activity != ActivityIdle {
		return state, nil
	}
	if hitch.PendingCompactID != "" {
		compact := state.Compactions[hitch.PendingCompactID]
		compact.Status = CompactionRunning
		state.Compactions[compact.ID] = compact
		hitch.Activity = ActivityCompacting
		state.Hitches[hitch.ID] = hitch
		return state, []Effect{CompactHitch{Compaction: compact, Pane: hitch.Pane}}
	}
	for _, id := range state.DeliveryOrder {
		delivery := state.Deliveries[id]
		if delivery.Status != DeliveryQueued || delivery.Envelope.To != hitch.Name || !delivery.NotBefore.IsZero() {
			continue
		}
		delivery.Status = DeliveryDelivering
		delivery.Reason = ""
		state.Deliveries[id] = delivery
		hitch.Activity = ActivityDelivering
		state.Hitches[hitch.ID] = hitch
		return state, []Effect{DeliverEnvelope{Envelope: delivery.Envelope, Pane: hitch.Pane, Deadline: delivery.Deadline}}
	}
	return state, nil
}

// DueTimedDeliveries returns scheduled envelopes ready at the supplied
// observed time. It is deterministic and does not read the clock itself.
func DueTimedDeliveries(state State, at time.Time) []EnvelopeID {
	var due []EnvelopeID
	for _, id := range state.DeliveryOrder {
		delivery := state.Deliveries[id]
		if delivery.Status == DeliveryQueued && !delivery.NotBefore.IsZero() && !at.Before(delivery.NotBefore) {
			due = append(due, id)
		}
	}
	return due
}

func invalidSender(state State, sender Sender) string {
	if sender.Name == "" {
		return "sender name is required"
	}
	switch sender.Kind {
	case SenderAgent:
		hitch, ok := activeHitchByID(state, sender.HitchID)
		if !ok || hitch.Name != sender.Name {
			return "agent sender does not match an active hitch"
		}
	case SenderSelfDeclared:
		if sender.HitchID != "" {
			return "self-declared sender cannot claim a hitch id"
		}
	default:
		return "sender kind is not recognized"
	}
	return ""
}

func activeDelivery(state State, id EnvelopeID) (Delivery, Hitch, bool) {
	delivery, ok := state.Deliveries[id]
	if !ok || delivery.Status != DeliveryDelivering {
		return Delivery{}, Hitch{}, false
	}
	hitch, ok := activeHitchByName(state, delivery.Envelope.To)
	if !ok || hitch.Activity != ActivityDelivering {
		return Delivery{}, Hitch{}, false
	}
	return delivery, hitch, true
}

func activeCompaction(state State, id CompactionID) (Compaction, Hitch, bool) {
	compact, ok := state.Compactions[id]
	if !ok || compact.Status != CompactionRunning {
		return Compaction{}, Hitch{}, false
	}
	hitch, ok := activeHitchByID(state, compact.HitchID)
	if !ok || hitch.Activity != ActivityCompacting || hitch.PendingCompactID != id {
		return Compaction{}, Hitch{}, false
	}
	return compact, hitch, true
}

func activeHitchByName(state State, name AgentName) (Hitch, bool) {
	for _, hitch := range state.Hitches {
		if hitch.Name == name && hitch.Status == HitchActive {
			return hitch, true
		}
	}
	return Hitch{}, false
}

func activeHitchByID(state State, id HitchID) (Hitch, bool) {
	hitch, ok := state.Hitches[id]
	return hitch, ok && hitch.Status == HitchActive
}

func validDeadline(at, deadline time.Time) bool {
	return !at.IsZero() && deadline.After(at)
}

func hitchOccupiesName(status HitchStatus) bool {
	return status == HitchStarting || status == HitchBooting || status == HitchActive || status == HitchDropping
}

func rejected(state State, event Event, at time.Time, reason string) (State, []Effect) {
	return state, []Effect{RecordEvent{Event: TransitionRejected{At: at, Event: EventName(event), Reason: reason}}}
}

func cloneState(state State) State {
	next := state
	next.Hitches = make(map[HitchID]Hitch, len(state.Hitches))
	for id, hitch := range state.Hitches {
		next.Hitches[id] = hitch
	}
	next.Deliveries = make(map[EnvelopeID]Delivery, len(state.Deliveries))
	for id, delivery := range state.Deliveries {
		next.Deliveries[id] = delivery
	}
	next.DeliveryOrder = append([]EnvelopeID(nil), state.DeliveryOrder...)
	next.Compactions = make(map[CompactionID]Compaction, len(state.Compactions))
	for id, compact := range state.Compactions {
		next.Compactions[id] = compact
	}
	return next
}

// PendingEffects reconstructs intents whose outcome is absent from the log.
// Callers execute them using the same idempotency checks as newly emitted
// effects, then append the observed outcome as another event.
func PendingEffects(state State) []Effect {
	ids := make([]string, 0, len(state.Hitches))
	for id := range state.Hitches {
		ids = append(ids, string(id))
	}
	sort.Strings(ids)

	var effects []Effect
	for _, rawID := range ids {
		hitch := state.Hitches[HitchID(rawID)]
		switch hitch.Status {
		case HitchStarting:
			effects = append(effects, SpawnHitch{Hitch: hitch})
		case HitchBooting:
			effects = append(effects, AwaitBoot{HitchID: hitch.ID, Pane: hitch.Pane, Deadline: hitch.BootDeadline})
		case HitchDropping:
			effects = append(effects, KillHitch{HitchID: hitch.ID, Pane: hitch.Pane, Deadline: hitch.DropDeadline})
		case HitchActive:
			switch hitch.Activity {
			case ActivityDelivering:
				for _, envelopeID := range state.DeliveryOrder {
					delivery := state.Deliveries[envelopeID]
					if delivery.Envelope.To == hitch.Name && delivery.Status == DeliveryDelivering {
						effects = append(effects, DeliverEnvelope{Envelope: delivery.Envelope, Pane: hitch.Pane, Deadline: delivery.Deadline})
						break
					}
				}
			case ActivityCompacting:
				compact := state.Compactions[hitch.PendingCompactID]
				if compact.Status == CompactionRunning {
					effects = append(effects, CompactHitch{Compaction: compact, Pane: hitch.Pane})
				}
			case ActivityInterrupting:
				effects = append(effects, InterruptHitch{HitchID: hitch.ID, Pane: hitch.Pane, Reason: hitch.InterruptReason, Deadline: hitch.InterruptDeadline})
			}
		}
	}
	return effects
}

func EventName(event Event) string {
	switch event.(type) {
	case HitchRequested:
		return "hitch_requested"
	case AdoptRequested:
		return "adopt_requested"
	case RenameRequested:
		return "rename_requested"
	case HitchSpawned:
		return "hitch_spawned"
	case HitchReady:
		return "hitch_ready"
	case HitchLaunchFailed:
		return "hitch_launch_failed"
	case TurnStarted:
		return "turn_started"
	case TurnBoundaryReached:
		return "turn_boundary_reached"
	case SendRequested:
		return "send_requested"
	case TimedDeliveryReleased:
		return "timed_delivery_released"
	case TimedDeliveriesCleared:
		return "timed_deliveries_cleared"
	case DeliverySucceeded:
		return "delivery_succeeded"
	case DeliveryDeferred:
		return "delivery_deferred"
	case DeliveryFailedEvent:
		return "delivery_failed"
	case DeliveryUnverifiedEvent:
		return "delivery_unverified"
	case CompactionRequested:
		return "compaction_requested"
	case CompactionCompleted:
		return "compaction_completed"
	case CompactionFailedEvent:
		return "compaction_failed"
	case InterruptRequested:
		return "interrupt_requested"
	case InterruptSucceeded:
		return "interrupt_succeeded"
	case InterruptFailed:
		return "interrupt_failed"
	case DropRequested:
		return "drop_requested"
	case DropSucceeded:
		return "drop_succeeded"
	case DropFailed:
		return "drop_failed"
	case WedgeDetected:
		return "wedge_detected"
	case WedgeCleared:
		return "wedge_cleared"
	case OperationTimedOut:
		return "operation_timed_out"
	case CurfewSet:
		return "curfew_set"
	case CurfewCleared:
		return "curfew_cleared"
	case TransitionRejected:
		return "transition_rejected"
	}
	return fmt.Sprintf("%T", event)
}
