package core

import "time"

func stepDropRequested(state State, event DropRequested) (State, []Effect) {
	hitch, ok := state.Hitches[event.HitchID]
	if !ok || (hitch.Status != HitchActive && hitch.Status != HitchFailed) {
		return rejected(state, event, event.At, "hitch is not active or failed")
	}
	if !validDeadline(event.At, event.Deadline) {
		return rejected(state, event, event.At, "drop deadline must be after event time")
	}
	hitch.PreviousStatus = hitch.Status
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
		delivery.Status = DeliveryFailed
		delivery.Reason = "recipient was dropped"
		state.Deliveries[id] = delivery
	}
	hitch.Pane = ""
	hitch.DropDeadline = time.Time{}
	hitch.InterruptDeadline = time.Time{}
	hitch.InterruptReason = ""
	hitch.BlockedEvidence = ""
	hitch.BlockedFrom = ""
	hitch.PreviousStatus = ""
	hitch.PreviousActivity = ""
	state.Hitches[hitch.ID] = hitch
	return state, nil
}

func stepDropFailed(state State, event DropFailed) (State, []Effect) {
	hitch, ok := state.Hitches[event.HitchID]
	if !ok || hitch.Status != HitchDropping || event.Reason == "" {
		return rejected(state, event, event.At, "hitch is not dropping or reason is empty")
	}
	hitch.Status = hitch.PreviousStatus
	if hitch.Status == "" {
		hitch.Status = HitchActive
	}
	hitch.Activity = hitch.PreviousActivity
	hitch.DropDeadline = time.Time{}
	hitch.PreviousStatus = ""
	hitch.PreviousActivity = ""
	state.Hitches[hitch.ID] = hitch
	return state, nil
}
