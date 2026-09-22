package core

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
	hitch.Activity = ActivityIdle
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
