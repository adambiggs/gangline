package core

import "time"

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
	if hitch.PendingCompactID != "" {
		compact := state.Compactions[hitch.PendingCompactID]
		compact.Status = CompactionCancelled
		compact.Reason = "hitch was dropped"
		state.Compactions[compact.ID] = compact
		hitch.PendingCompactID = ""
	}
	for id, delivery := range state.Deliveries {
		if delivery.Envelope.To != hitch.Name || (delivery.Status != DeliveryQueued && delivery.Status != DeliveryDelivering) {
			continue
		}
		delivery.Status = DeliveryCancelled
		delivery.Reason = "recipient was dropped"
		state.Deliveries[id] = delivery
	}
	hitch.Pane = ""
	hitch.DropDeadline = time.Time{}
	hitch.InterruptDeadline = time.Time{}
	hitch.InterruptReason = ""
	hitch.BlockedEvidence = ""
	hitch.BlockedFrom = ""
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
	hitch.DropDeadline = time.Time{}
	hitch.PreviousActivity = ""
	state.Hitches[hitch.ID] = hitch
	return state, nil
}
