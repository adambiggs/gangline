package core

import "time"

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
	return status == HitchStarting || status == HitchBooting || status == HitchActive || status == HitchDropping || status == HitchFailed
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
