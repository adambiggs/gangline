package core

import (
	"crypto/sha256"
	"fmt"
	"time"
)

// CapacityRecovery belongs to the explicit recovery command. The log preserves
// its schedule across invocations; no process watches it between commands.
type CapacityRecovery struct {
	Fingerprint string     `json:"fingerprint,omitempty"`
	Evidence    string     `json:"evidence,omitempty"`
	Deadline    time.Time  `json:"deadline,omitzero"`
	NextAt      time.Time  `json:"next_at,omitzero"`
	Attempts    int        `json:"attempts,omitempty"`
	EnvelopeID  EnvelopeID `json:"envelope_id,omitempty"`
	Expired     bool       `json:"expired,omitempty"`
}

type CapacityDetected struct {
	At          time.Time `json:"at"`
	HitchID     HitchID   `json:"hitch_id"`
	Fingerprint string    `json:"fingerprint"`
	Evidence    string    `json:"evidence"`
	Deadline    time.Time `json:"deadline"`
}

type CapacityRetryRequested struct {
	At          time.Time `json:"at"`
	HitchID     HitchID   `json:"hitch_id"`
	Fingerprint string    `json:"fingerprint"`
}

type CapacityExpired struct {
	At      time.Time `json:"at"`
	HitchID HitchID   `json:"hitch_id"`
}

type CapacityCleared struct {
	At      time.Time `json:"at"`
	HitchID HitchID   `json:"hitch_id"`
}

func (CapacityDetected) isEvent()       {}
func (CapacityRetryRequested) isEvent() {}
func (CapacityExpired) isEvent()        {}
func (CapacityCleared) isEvent()        {}

// RetryDelay is shared by safe delivery deferrals and provider recovery.
func RetryDelay(attempt int) time.Duration {
	delay := 100 * time.Millisecond
	for attempt > 0 && delay < 30*time.Second {
		delay = min(delay*2, 30*time.Second)
		attempt--
	}
	return delay
}

func stepCapacityDetected(state State, event CapacityDetected) (State, []Effect) {
	hitch, ok := activeHitchByID(state, event.HitchID)
	if !ok || (hitch.Activity != ActivityBusy && hitch.Activity != ActivityIdle) || deliveryInProgress(state, hitch.Name) || hitch.PendingCompactID != "" || event.Fingerprint == "" || event.Evidence == "" || !validDeadline(event.At, event.Deadline) {
		return rejected(state, event, event.At, "capacity observation does not name an observable active turn")
	}
	if hitch.Capacity.Fingerprint == event.Fingerprint {
		return state, nil
	}
	if hitch.Capacity.Deadline.IsZero() {
		hitch.Capacity.Deadline = event.Deadline
	}
	cancelCapacityDelivery(state, hitch, "superseded by a later terminal capacity failure")
	hitch.Capacity.Fingerprint = event.Fingerprint
	hitch.Capacity.Evidence = event.Evidence
	hitch.Capacity.EnvelopeID = ""
	hitch.Capacity.NextAt = event.At.Add(RetryDelay(hitch.Capacity.Attempts))
	hitch.Activity = ActivityIdle
	state.Hitches[hitch.ID] = hitch
	return dispatchNext(state, hitch.ID)
}

func stepCapacityRetryRequested(state State, event CapacityRetryRequested) (State, []Effect) {
	hitch, ok := activeHitchByID(state, event.HitchID)
	if !ok || hitch.Activity != ActivityIdle || hitch.PendingCompactID != "" || deliveryInProgress(state, hitch.Name) || hitch.Capacity.Fingerprint != event.Fingerprint || event.Fingerprint == "" || hitch.Capacity.Expired || event.At.Before(hitch.Capacity.NextAt) || !event.At.Before(hitch.Capacity.Deadline) {
		return rejected(state, event, event.At, "capacity recovery is not due or input is unavailable")
	}
	// User input takes priority and itself resumes the failed turn.
	for _, delivery := range state.Deliveries {
		if delivery.Envelope.To == hitch.Name && delivery.Status == DeliveryQueued && !delivery.CapacityRecovery && delivery.NotBefore.IsZero() {
			return dispatchNext(state, hitch.ID)
		}
	}
	id := hitch.Capacity.EnvelopeID
	if id == "" {
		digest := sha256.Sum256([]byte(string(hitch.ID) + ":" + event.Fingerprint))
		id = EnvelopeID(fmt.Sprintf("capacity-%x", digest[:16]))
		if _, exists := state.Deliveries[id]; exists {
			return rejected(state, event, event.At, "capacity continuation already recorded")
		}
		state.Deliveries[id] = Delivery{
			Envelope: Envelope{ID: id, From: Sender{Kind: SenderSelfDeclared, Name: "capacity-recovery"}, To: hitch.Name, Message: Message{Text: "Continue the interrupted work after the provider capacity error."}, CreatedAt: event.At},
			Status:   DeliveryQueued, CapacityRecovery: true,
		}
		state.DeliveryOrder = append(state.DeliveryOrder, id)
	}
	if state.Deliveries[id].Status != DeliveryQueued {
		return rejected(state, event, event.At, "capacity input was already submitted or its outcome is unknown")
	}
	hitch.Capacity.EnvelopeID = id
	hitch.Capacity.Attempts++
	hitch.Capacity.NextAt = event.At.Add(RetryDelay(hitch.Capacity.Attempts))
	state.Hitches[hitch.ID] = hitch
	return stepDeliveryRetryRequested(state, DeliveryRetryRequested{At: event.At, EnvelopeID: id})
}

func stepCapacityExpired(state State, event CapacityExpired) (State, []Effect) {
	hitch, ok := activeHitchByID(state, event.HitchID)
	if !ok || hitch.Capacity.Fingerprint == "" || event.At.Before(hitch.Capacity.Deadline) {
		return rejected(state, event, event.At, "capacity recovery has not reached its deadline")
	}
	hitch.Capacity.Expired = true
	cancelCapacityDelivery(state, hitch, "provider capacity recovery deadline elapsed")
	if hitch.Activity == ActivityIdle {
		hitch.Activity = ActivityWedged
		hitch.WedgeEvidence = "provider capacity recovery deadline elapsed: " + hitch.Capacity.Evidence
	}
	state.Hitches[hitch.ID] = hitch
	return state, nil
}

func stepCapacityCleared(state State, event CapacityCleared) (State, []Effect) {
	hitch, ok := activeHitchByID(state, event.HitchID)
	if !ok || hitch.Capacity.Fingerprint == "" {
		return state, nil
	}
	cancelCapacityDelivery(state, hitch, "native activity ended provider capacity recovery")
	hitch.Capacity = CapacityRecovery{}
	state.Hitches[hitch.ID] = hitch
	return state, nil
}

func cancelCapacityDelivery(state State, hitch Hitch, reason string) {
	id := hitch.Capacity.EnvelopeID
	if delivery, ok := state.Deliveries[id]; ok && delivery.Status == DeliveryQueued {
		delivery.Status, delivery.Reason = DeliveryCancelled, reason
		state.Deliveries[id] = delivery
	}
}
