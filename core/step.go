package core

import "time"

func Step(agent Agent, event Event) (Agent, []Effect) {
	if event.HitchID != agent.ID || event.At.IsZero() {
		return agent, []Effect{{Kind: "reject", Reason: "event does not identify this agent and its time"}}
	}
	if agent.Compaction != nil {
		c := *agent.Compaction
		agent.Compaction = &c
	}
	switch event.Type {
	case "hitch_spawned":
		agent.Pane, agent.Status = event.Pane, Booting
	case "hitch_ready":
		agent.Status, agent.Activity, agent.BootDeadline = Active, Idle, time.Time{}
	case "hitch_blocked":
		agent.Activity, agent.Evidence, agent.BootDeadline = Blocked, event.Reason, time.Time{}
	case "hitch_failed":
		agent.Status, agent.Activity, agent.Evidence = Failed, Unknown, event.Reason
	case "activity_observed":
		if agent.Status == Active {
			agent.Activity, agent.Evidence = event.Activity, event.Reason
		}
	case "input_started":
		if agent.Input != nil {
			return agent, []Effect{{Kind: "reject", Reason: "input already in progress"}}
		}
		agent.Input = &InputIntent{ID: event.ID, Kind: event.Status, At: event.At}
	case "input_finished":
		if agent.Input == nil || agent.Input.ID != event.ID {
			return agent, []Effect{{Kind: "reject", Reason: "input result has no matching intent"}}
		}
		agent.Input = nil
		if event.Status == "delivered" {
			agent.Activity, agent.Native.SubmittedAt = Busy, event.At
		} else if event.Status == "unverified" {
			agent.Activity, agent.Evidence = Wedged, event.Reason
		}
	case "compaction_requested":
		if agent.Compaction != nil && (agent.Compaction.Status == "queued" || agent.Compaction.Status == "submitted" && !agent.Compaction.Continuation) {
			return agent, []Effect{{Kind: "reject", Reason: "compaction already pending"}}
		}
		if event.Compaction == nil {
			return agent, []Effect{{Kind: "reject", Reason: "compaction is required"}}
		}
		c := *event.Compaction
		agent.Compaction = &c
	case "compaction_submitted":
		if agent.Compaction == nil || agent.Compaction.ID != event.ID {
			return agent, nil
		}
		agent.Compaction.Status, agent.Activity, agent.Input = "submitted", Compacting, nil
	case "compaction_unverified":
		if agent.Compaction == nil || agent.Compaction.ID != event.ID {
			return agent, nil
		}
		agent.Compaction.Status, agent.Activity, agent.Evidence, agent.Input = "unverified", Wedged, event.Reason, nil
	case "interrupt_requested":
		agent.Activity, agent.InterruptDeadline = Interrupting, event.Deadline
	case "interrupt_completed":
		agent.Activity, agent.InterruptDeadline = Idle, time.Time{}
	case "drop_started":
		agent.Status = Dropping
	case "capacity_detected":
		if agent.Capacity.Fingerprint != event.Fingerprint {
			if agent.Capacity.Deadline.IsZero() {
				agent.Capacity.Deadline = event.Deadline
			}
			agent.Capacity.Fingerprint, agent.Capacity.Evidence = event.Fingerprint, event.Reason
			agent.Capacity.NextAt = event.At.Add(RetryDelay(agent.Capacity.Attempts))
			agent.Capacity.Submitted = false
		}
	case "capacity_submitted":
		agent.Capacity.Submitted = true
		agent.Capacity.Attempts++
	case "capacity_cleared":
		agent.Capacity = Capacity{}
	case "deadline_checked":
		if agent.Status == Dropping {
			return agent, nil
		}
		if !agent.BootDeadline.IsZero() && !event.At.Before(agent.BootDeadline) && (agent.Status == Starting || agent.Status == Booting) {
			agent.Status, agent.Evidence = Failed, "boot deadline elapsed"
		}
		if agent.Activity == Interrupting && !agent.InterruptDeadline.IsZero() && !event.At.Before(agent.InterruptDeadline) {
			agent.Activity, agent.Evidence = Wedged, "interrupt deadline elapsed"
		}
		if !agent.Capacity.Deadline.IsZero() && !event.At.Before(agent.Capacity.Deadline) {
			agent.Activity, agent.Evidence = Wedged, "provider capacity recovery deadline elapsed"
		}
	default:
		return agent, nil
	}
	agent.ChangedAt = event.At
	return agent, nil
}
