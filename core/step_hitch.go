package core

import "time"

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
