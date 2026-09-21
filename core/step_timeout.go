package core

import "time"

func stepTimedOut(state State, event OperationTimedOut) (State, []Effect) {
	if event.Evidence == "" {
		return rejected(state, event, event.At, "timeout evidence is empty")
	}
	switch event.Operation {
	case TimeoutBoot:
		hitch, ok := state.Hitches[HitchID(event.ID)]
		if !ok || (hitch.Status != HitchStarting && hitch.Status != HitchBooting) || !event.Deadline.Equal(hitch.BootDeadline) {
			return rejected(state, event, event.At, "boot timeout does not match a pending boot")
		}
		hitch.Status = HitchFailed
		hitch.Activity = ActivityUnknown
		hitch.BootDeadline = time.Time{}
		hitch.WedgeEvidence = event.Evidence
		state.Hitches[hitch.ID] = hitch
		return state, nil
	case TimeoutTurn:
		hitch, ok := activeHitchByID(state, HitchID(event.ID))
		if !ok || hitch.Activity != ActivityBusy {
			return rejected(state, event, event.At, "turn timeout does not match a busy hitch")
		}
		hitch.Activity = ActivityWedged
		hitch.WedgeEvidence = event.Evidence
		state.Hitches[hitch.ID] = hitch
		return state, nil
	case TimeoutDelivery:
		delivery, hitch, ok := activeDelivery(state, EnvelopeID(event.ID))
		if !ok || !event.Deadline.Equal(delivery.Deadline) {
			return rejected(state, event, event.At, "delivery timeout does not match an in-progress delivery")
		}
		delivery.Status = DeliveryUnverified
		delivery.Reason = event.Evidence
		state.Deliveries[delivery.Envelope.ID] = delivery
		hitch.Activity = ActivityWedged
		hitch.WedgeEvidence = event.Evidence
		state.Hitches[hitch.ID] = hitch
		return state, nil
	case TimeoutCompaction:
		compact, hitch, ok := activeCompaction(state, CompactionID(event.ID))
		if !ok || !event.Deadline.Equal(compact.Deadline) {
			return rejected(state, event, event.At, "compaction timeout does not match an in-progress compaction")
		}
		compact.Status = CompactionUnverified
		compact.Reason = event.Evidence
		state.Compactions[compact.ID] = compact
		hitch.PendingCompactID = ""
		hitch.Activity = ActivityWedged
		hitch.WedgeEvidence = event.Evidence
		state.Hitches[hitch.ID] = hitch
		return state, nil
	case TimeoutInterrupt:
		hitch, ok := activeHitchByID(state, HitchID(event.ID))
		if !ok || hitch.Activity != ActivityInterrupting || !event.Deadline.Equal(hitch.InterruptDeadline) {
			return rejected(state, event, event.At, "interrupt timeout does not match an in-progress interrupt")
		}
		hitch.Activity = ActivityWedged
		hitch.InterruptDeadline = time.Time{}
		hitch.InterruptReason = ""
		hitch.PreviousActivity = ""
		hitch.WedgeEvidence = event.Evidence
		state.Hitches[hitch.ID] = hitch
		return state, nil
	case TimeoutDrop:
		hitch, ok := state.Hitches[HitchID(event.ID)]
		if !ok || hitch.Status != HitchDropping || !event.Deadline.Equal(hitch.DropDeadline) {
			return rejected(state, event, event.At, "drop timeout does not match a pending drop")
		}
		hitch.Status = HitchActive
		hitch.Activity = ActivityWedged
		hitch.DropDeadline = time.Time{}
		hitch.BlockedEvidence = ""
		hitch.BlockedFrom = ""
		hitch.PreviousActivity = ""
		hitch.WedgeEvidence = event.Evidence
		state.Hitches[hitch.ID] = hitch
		return state, nil
	default:
		return rejected(state, event, event.At, "unknown timeout operation")
	}
}

func stepCurfewSet(state State, event CurfewSet) (State, []Effect) {
	if !validDeadline(event.At, event.Deadline) {
		return rejected(state, event, event.At, "curfew deadline must be after event time")
	}
	state.Team.Curfew = event.Deadline
	return state, nil
}

func stepCurfewCleared(state State, event CurfewCleared) (State, []Effect) {
	if state.Team.Curfew.IsZero() {
		return rejected(state, event, event.At, "team has no curfew")
	}
	state.Team.Curfew = time.Time{}
	return state, nil
}
