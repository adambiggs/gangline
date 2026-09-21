package core

import "time"

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
