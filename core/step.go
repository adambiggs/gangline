package core

import (
	"fmt"
	"time"
)

func Step(state State, event Event) (State, []Effect) {
	next := cloneState(state)

	switch event := event.(type) {
	case HitchRequested:
		if _, exists := next.Hitches[event.Hitch.ID]; exists {
			return next, reject(event.At, eventName(event), "hitch id already exists")
		}
		if event.Hitch.ID == "" || event.Hitch.Name == "" {
			return next, reject(event.At, eventName(event), "hitch id and name are required")
		}
		for _, hitch := range next.Hitches {
			if hitch.Name == event.Hitch.Name && hitchOccupiesName(hitch.Status) {
				return next, reject(event.At, eventName(event), "hitch name already exists")
			}
		}
		hitch := event.Hitch
		hitch.Status = HitchStarting
		next.Hitches[hitch.ID] = hitch
		return next, []Effect{SpawnHitch{Hitch: hitch}}

	case HitchReady:
		hitch, ok := next.Hitches[event.HitchID]
		if !ok || hitch.Status != HitchStarting || event.Pane == "" {
			return next, reject(event.At, eventName(event), "hitch is not starting or pane is empty")
		}
		hitch.Status = HitchActive
		hitch.Pane = event.Pane
		next.Hitches[event.HitchID] = hitch
		return next, nil

	case HitchLaunchFailed:
		hitch, ok := next.Hitches[event.HitchID]
		if !ok || hitch.Status != HitchStarting {
			return next, reject(event.At, eventName(event), "hitch is not starting")
		}
		hitch.Status = HitchFailed
		next.Hitches[event.HitchID] = hitch
		return next, nil

	case SendRequested:
		if _, exists := next.Deliveries[event.Envelope.ID]; exists {
			return next, reject(event.At, eventName(event), "envelope id already exists")
		}
		hitch, ok := activeHitch(next, event.Envelope.To)
		if !ok || event.Envelope.ID == "" {
			return next, reject(event.At, eventName(event), "recipient is not active or envelope id is empty")
		}
		next.Deliveries[event.Envelope.ID] = Delivery{
			Envelope: event.Envelope,
			Status:   DeliveryPending,
		}
		return next, []Effect{DeliverEnvelope{Envelope: event.Envelope, Pane: hitch.Pane}}

	case DeliverySucceeded:
		delivery, ok := next.Deliveries[event.EnvelopeID]
		if !ok || delivery.Status != DeliveryPending {
			return next, reject(event.At, eventName(event), "delivery is not pending")
		}
		delivery.Status = DeliveryDelivered
		next.Deliveries[event.EnvelopeID] = delivery
		return next, nil

	case DeliveryFailedEvent:
		delivery, ok := next.Deliveries[event.EnvelopeID]
		if !ok || delivery.Status != DeliveryPending {
			return next, reject(event.At, eventName(event), "delivery is not pending")
		}
		delivery.Status = DeliveryFailed
		delivery.Reason = event.Reason
		next.Deliveries[event.EnvelopeID] = delivery
		return next, nil

	case DropRequested:
		hitch, ok := next.Hitches[event.HitchID]
		if !ok || hitch.Status != HitchActive {
			return next, reject(event.At, eventName(event), "hitch is not active")
		}
		hitch.Status = HitchDropping
		next.Hitches[event.HitchID] = hitch
		return next, []Effect{KillHitch{HitchID: hitch.ID, Pane: hitch.Pane}}

	case DropSucceeded:
		hitch, ok := next.Hitches[event.HitchID]
		if !ok || hitch.Status != HitchDropping {
			return next, reject(event.At, eventName(event), "hitch is not dropping")
		}
		hitch.Status = HitchDropped
		hitch.Pane = ""
		next.Hitches[event.HitchID] = hitch
		return next, nil

	case DropFailed:
		hitch, ok := next.Hitches[event.HitchID]
		if !ok || hitch.Status != HitchDropping {
			return next, reject(event.At, eventName(event), "hitch is not dropping")
		}
		hitch.Status = HitchActive
		next.Hitches[event.HitchID] = hitch
		return next, nil

	case TransitionRejected:
		return next, nil
	}

	return next, nil
}

func hitchOccupiesName(status HitchStatus) bool {
	return status == HitchStarting || status == HitchActive || status == HitchDropping
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
	return next
}

func activeHitch(state State, name AgentName) (Hitch, bool) {
	for _, hitch := range state.Hitches {
		if hitch.Name == name && hitch.Status == HitchActive {
			return hitch, true
		}
	}
	return Hitch{}, false
}

func reject(at time.Time, name, reason string) []Effect {
	return []Effect{RecordEvent{Event: TransitionRejected{At: at, Event: name, Reason: reason}}}
}

func eventName(event Event) string {
	switch event.(type) {
	case HitchRequested:
		return "hitch_requested"
	case HitchReady:
		return "hitch_ready"
	case HitchLaunchFailed:
		return "hitch_launch_failed"
	case SendRequested:
		return "send_requested"
	case DeliverySucceeded:
		return "delivery_succeeded"
	case DeliveryFailedEvent:
		return "delivery_failed"
	case DropRequested:
		return "drop_requested"
	case DropSucceeded:
		return "drop_succeeded"
	case DropFailed:
		return "drop_failed"
	case TransitionRejected:
		return "transition_rejected"
	}
	return fmt.Sprintf("%T", event)
}
