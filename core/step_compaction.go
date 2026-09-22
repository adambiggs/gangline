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
	if envelope := event.Continuation; envelope != nil {
		_, exists := state.Deliveries[envelope.ID]
		if exists || envelope.ID == "" || envelope.To != hitch.Name || envelope.Message != compact.Resume ||
			envelope.From != (Sender{Kind: SenderSelfDeclared, Name: "compact"}) || event.At.IsZero() {
			return rejected(state, event, event.At, "compaction continuation does not match its request")
		}
	}
	compact.Status = CompactionSucceeded
	state.Compactions[compact.ID] = compact
	hitch.PendingCompactID = ""
	hitch.Activity = ActivityIdle
	state.Hitches[hitch.ID] = hitch
	if event.Continuation != nil {
		return stepSendRequested(state, SendRequested{At: event.At, Envelope: *event.Continuation})
	}
	return state, nil
}

func stepCompactionSubmitted(state State, event CompactionSubmitted) (State, []Effect) {
	compact, _, ok := activeCompaction(state, event.CompactionID)
	if !ok {
		// A fast completion hook may already have settled the compaction.
		return state, nil
	}
	compact.Submitted = true
	state.Compactions[compact.ID] = compact
	return state, nil
}

func stepCompactionUnverified(state State, event CompactionUnverifiedEvent) (State, []Effect) {
	compact, hitch, ok := activeCompaction(state, event.CompactionID)
	if !ok || event.Evidence == "" {
		return rejected(state, event, event.At, "compaction is not in progress or evidence is empty")
	}
	compact.Status, compact.Reason = CompactionUnverified, event.Evidence
	state.Compactions[compact.ID] = compact
	hitch.PendingCompactID = ""
	hitch.Activity, hitch.WedgeEvidence = ActivityWedged, event.Evidence
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
