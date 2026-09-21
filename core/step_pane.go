package core

import "time"

func stepPaneVanished(state State, event PaneVanished) (State, []Effect) {
	hitch, ok := activeHitchByID(state, event.HitchID)
	if !ok || event.Evidence == "" {
		return rejected(state, event, event.At, "hitch is not active or evidence is empty")
	}

	hitch.Status = HitchFailed
	hitch.Activity = ActivityWedged
	hitch.BootDeadline = time.Time{}
	hitch.DropDeadline = time.Time{}
	hitch.InterruptDeadline = time.Time{}
	hitch.InterruptReason = ""
	hitch.BlockedEvidence = ""
	hitch.BlockedFrom = ""
	hitch.PreviousActivity = ""
	hitch.WedgeEvidence = event.Evidence

	if hitch.PendingCompactID != "" {
		compact := state.Compactions[hitch.PendingCompactID]
		if compact.Status == CompactionRunning {
			compact.Status = CompactionUnverified
		} else {
			compact.Status = CompactionFailed
		}
		compact.Reason = event.Evidence
		state.Compactions[compact.ID] = compact
		hitch.PendingCompactID = ""
	}

	for id, delivery := range state.Deliveries {
		if delivery.Envelope.To != hitch.Name {
			continue
		}
		switch delivery.Status {
		case DeliveryQueued:
			delivery.Status = DeliveryFailed
			delivery.Reason = event.Evidence
		case DeliveryDelivering:
			delivery.Status = DeliveryUnverified
			delivery.Reason = event.Evidence
		default:
			continue
		}
		state.Deliveries[id] = delivery
	}

	state.Hitches[hitch.ID] = hitch
	return state, nil
}
