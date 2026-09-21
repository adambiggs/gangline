package core

import "sort"

// PendingEffects reconstructs intents whose outcome is absent from the log.
// Callers execute them using the same idempotency checks as newly emitted
// effects, then append the observed outcome as another event.
func PendingEffects(state State) []Effect {
	ids := make([]string, 0, len(state.Hitches))
	for id := range state.Hitches {
		ids = append(ids, string(id))
	}
	sort.Strings(ids)

	var effects []Effect
	for _, rawID := range ids {
		hitch := state.Hitches[HitchID(rawID)]
		switch hitch.Status {
		case HitchStarting:
			effects = append(effects, SpawnHitch{Hitch: hitch})
		case HitchBooting:
			effects = append(effects, AwaitBoot{HitchID: hitch.ID, Pane: hitch.Pane, Deadline: hitch.BootDeadline})
		case HitchDropping:
			effects = append(effects, KillHitch{HitchID: hitch.ID, Pane: hitch.Pane, Deadline: hitch.DropDeadline})
		case HitchActive:
			switch hitch.Activity {
			case ActivityDelivering:
				for _, envelopeID := range state.DeliveryOrder {
					delivery := state.Deliveries[envelopeID]
					if delivery.Envelope.To == hitch.Name && delivery.Status == DeliveryDelivering {
						effects = append(effects, DeliverEnvelope{Envelope: delivery.Envelope, Pane: hitch.Pane, Deadline: delivery.Deadline})
						break
					}
				}
			case ActivityCompacting:
				compact := state.Compactions[hitch.PendingCompactID]
				if compact.Status == CompactionRunning {
					effects = append(effects, CompactHitch{Compaction: compact, Pane: hitch.Pane})
				}
			case ActivityInterrupting:
				effects = append(effects, InterruptHitch{HitchID: hitch.ID, Pane: hitch.Pane, Reason: hitch.InterruptReason, Deadline: hitch.InterruptDeadline})
			}
		}
	}
	return effects
}
