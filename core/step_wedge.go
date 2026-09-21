package core

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
