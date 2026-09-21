package core

func stepBlockedDetected(state State, event BlockedDetected) (State, []Effect) {
	hitch, ok := activeHitchByID(state, event.HitchID)
	if !ok || event.Evidence == "" || hitch.Activity == ActivityBlocked || hitch.Activity == ActivityUnknown || hitch.Activity == ActivityWedged {
		return rejected(state, event, event.At, "hitch cannot enter blocked state or evidence is empty")
	}
	previous := hitch.Activity
	if hitch.Activity == ActivityIdle {
		previous = ActivityBusy
	}
	if hitch.Activity == ActivityDelivering {
		previous = ActivityBusy
		for id, delivery := range state.Deliveries {
			if delivery.Envelope.To != hitch.Name || delivery.Status != DeliveryDelivering {
				continue
			}
			delivery.Status = DeliveryQueued
			delivery.Reason = event.Evidence
			state.Deliveries[id] = delivery
		}
	}
	hitch.BlockedFrom = previous
	hitch.Activity = ActivityBlocked
	hitch.BlockedEvidence = event.Evidence
	state.Hitches[hitch.ID] = hitch
	return state, nil
}

func stepBlockedCleared(state State, event BlockedCleared) (State, []Effect) {
	hitch, ok := activeHitchByID(state, event.HitchID)
	if !ok || hitch.Activity != ActivityBlocked {
		return rejected(state, event, event.At, "hitch is not blocked")
	}
	activity := hitch.BlockedFrom
	if activity == "" || activity == ActivityDelivering || activity == ActivityBlocked {
		activity = ActivityBusy
	}
	hitch.Activity = activity
	hitch.BlockedFrom = ""
	hitch.BlockedEvidence = ""
	state.Hitches[hitch.ID] = hitch
	if activity == ActivityIdle {
		return dispatchNext(state, hitch.ID)
	}
	return state, nil
}
