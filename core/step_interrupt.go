package core

import "time"

func stepInterruptRequested(state State, event InterruptRequested) (State, []Effect) {
	hitch, ok := activeHitchByID(state, event.HitchID)
	if !ok || (hitch.Activity != ActivityBusy && hitch.Activity != ActivityWedged) {
		return rejected(state, event, event.At, "hitch is not in an interruptible turn")
	}
	if !validDeadline(event.At, event.Deadline) {
		return rejected(state, event, event.At, "interrupt deadline must be after event time")
	}
	hitch.PreviousActivity = hitch.Activity
	hitch.Activity = ActivityInterrupting
	hitch.InterruptDeadline = event.Deadline
	hitch.InterruptReason = event.Reason
	state.Hitches[hitch.ID] = hitch
	return state, []Effect{InterruptHitch{HitchID: hitch.ID, Pane: hitch.Pane, Reason: event.Reason, Deadline: event.Deadline}}
}

func stepInterruptSucceeded(state State, event InterruptSucceeded) (State, []Effect) {
	hitch, ok := activeHitchByID(state, event.HitchID)
	if !ok || hitch.Activity != ActivityInterrupting {
		return rejected(state, event, event.At, "interrupt is not in progress")
	}
	hitch.Activity = ActivityBusy
	hitch.InterruptDeadline = time.Time{}
	hitch.InterruptReason = ""
	hitch.PreviousActivity = ""
	state.Hitches[hitch.ID] = hitch
	return state, nil
}

func stepInterruptFailed(state State, event InterruptFailed) (State, []Effect) {
	hitch, ok := activeHitchByID(state, event.HitchID)
	if !ok || hitch.Activity != ActivityInterrupting || event.Reason == "" {
		return rejected(state, event, event.At, "interrupt is not in progress or reason is empty")
	}
	hitch.Activity = hitch.PreviousActivity
	hitch.InterruptDeadline = time.Time{}
	hitch.InterruptReason = ""
	hitch.PreviousActivity = ""
	state.Hitches[hitch.ID] = hitch
	return state, nil
}
