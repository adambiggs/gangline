package core

import "time"

//sumtype:decl
type Event interface {
	isEvent()
}

type HitchRequested struct {
	At           time.Time `json:"at"`
	Hitch        Hitch     `json:"hitch"`
	BootDeadline time.Time `json:"boot_deadline"`
}

type AdoptRequested struct {
	At    time.Time `json:"at"`
	Hitch Hitch     `json:"hitch"`
	Pane  string    `json:"pane"`
}

type RenameRequested struct {
	At      time.Time `json:"at"`
	HitchID HitchID   `json:"hitch_id"`
	Name    AgentName `json:"name"`
}

type HitchSpawned struct {
	At      time.Time `json:"at"`
	HitchID HitchID   `json:"hitch_id"`
	Pane    string    `json:"pane"`
}

type HitchReady struct {
	At      time.Time `json:"at"`
	HitchID HitchID   `json:"hitch_id"`
}

type HitchLaunchFailed struct {
	At      time.Time `json:"at"`
	HitchID HitchID   `json:"hitch_id"`
	Reason  string    `json:"reason"`
}

type TurnStarted struct {
	At      time.Time `json:"at"`
	HitchID HitchID   `json:"hitch_id"`
}

type TurnBoundaryReached struct {
	At      time.Time `json:"at"`
	HitchID HitchID   `json:"hitch_id"`
}

type BlockedDetected struct {
	At       time.Time `json:"at"`
	HitchID  HitchID   `json:"hitch_id"`
	Evidence string    `json:"evidence"`
}

type BlockedCleared struct {
	At      time.Time `json:"at"`
	HitchID HitchID   `json:"hitch_id"`
}

type SendRequested struct {
	MidTurn   bool      `json:"mid_turn,omitempty"`
	At        time.Time `json:"at"`
	Envelope  Envelope  `json:"envelope"`
	Deadline  time.Time `json:"deadline"`
	NotBefore time.Time `json:"not_before,omitempty"`
}

type TimedDeliveryReleased struct {
	At         time.Time  `json:"at"`
	EnvelopeID EnvelopeID `json:"envelope_id"`
}

type TimedDeliveriesCleared struct {
	At        time.Time `json:"at"`
	Recipient AgentName `json:"recipient"`
}

type DeliverySucceeded struct {
	At         time.Time  `json:"at"`
	EnvelopeID EnvelopeID `json:"envelope_id"`
}

// DeliveryRetryRequested records a direct empty-composer observation that
// makes one queued envelope safe to attempt again.
type DeliveryRetryRequested struct {
	At         time.Time  `json:"at"`
	EnvelopeID EnvelopeID `json:"envelope_id"`
}

// DeliveryDeferred is a known refusal before any input was sent. It is safe to
// retry the envelope at a later native turn boundary.
type DeliveryDeferred struct {
	At         time.Time  `json:"at"`
	EnvelopeID EnvelopeID `json:"envelope_id"`
	Reason     string     `json:"reason"`
}

type DeliveryFailedEvent struct {
	At         time.Time  `json:"at"`
	EnvelopeID EnvelopeID `json:"envelope_id"`
	Reason     string     `json:"reason"`
}

// DeliveryUnverifiedEvent means input may have landed. It is terminal and is
// never retried automatically.
type DeliveryUnverifiedEvent struct {
	At         time.Time  `json:"at"`
	EnvelopeID EnvelopeID `json:"envelope_id"`
	Evidence   string     `json:"evidence"`
}

type CompactionRequested struct {
	At         time.Time  `json:"at"`
	Compaction Compaction `json:"compaction"`
}

type CompactionCompleted struct {
	At           time.Time    `json:"at"`
	CompactionID CompactionID `json:"compaction_id"`
}

type CompactionFailedEvent struct {
	At           time.Time    `json:"at"`
	CompactionID CompactionID `json:"compaction_id"`
	Reason       string       `json:"reason"`
}

type InterruptRequested struct {
	At       time.Time `json:"at"`
	HitchID  HitchID   `json:"hitch_id"`
	Reason   string    `json:"reason,omitempty"`
	Deadline time.Time `json:"deadline"`
}

type InterruptSucceeded struct {
	At      time.Time `json:"at"`
	HitchID HitchID   `json:"hitch_id"`
}

type InterruptFailed struct {
	At      time.Time `json:"at"`
	HitchID HitchID   `json:"hitch_id"`
	Reason  string    `json:"reason"`
}

type DropRequested struct {
	At       time.Time `json:"at"`
	HitchID  HitchID   `json:"hitch_id"`
	Deadline time.Time `json:"deadline"`
}

type DropSucceeded struct {
	At      time.Time `json:"at"`
	HitchID HitchID   `json:"hitch_id"`
}

type DropFailed struct {
	At      time.Time `json:"at"`
	HitchID HitchID   `json:"hitch_id"`
	Reason  string    `json:"reason"`
}

type PaneVanished struct {
	At       time.Time `json:"at"`
	HitchID  HitchID   `json:"hitch_id"`
	Evidence string    `json:"evidence"`
}

type WedgeDetected struct {
	At       time.Time `json:"at"`
	HitchID  HitchID   `json:"hitch_id"`
	Evidence string    `json:"evidence"`
}

type WedgeCleared struct {
	At      time.Time `json:"at"`
	HitchID HitchID   `json:"hitch_id"`
}

type TimeoutOperation string

const (
	TimeoutBoot       TimeoutOperation = "boot"
	TimeoutTurn       TimeoutOperation = "turn"
	TimeoutDelivery   TimeoutOperation = "delivery"
	TimeoutCompaction TimeoutOperation = "compaction"
	TimeoutInterrupt  TimeoutOperation = "interrupt"
	TimeoutDrop       TimeoutOperation = "drop"
	TimeoutWait       TimeoutOperation = "wait"
)

type OperationTimedOut struct {
	At        time.Time        `json:"at"`
	Operation TimeoutOperation `json:"operation"`
	ID        string           `json:"id"`
	Deadline  time.Time        `json:"deadline"`
	Evidence  string           `json:"evidence"`
}

type CurfewSet struct {
	At       time.Time `json:"at"`
	Deadline time.Time `json:"deadline"`
}

type CurfewCleared struct {
	At time.Time `json:"at"`
}

type TransitionRejected struct {
	At     time.Time `json:"at"`
	Event  string    `json:"event"`
	Reason string    `json:"reason"`
}

func (HitchRequested) isEvent()          {}
func (AdoptRequested) isEvent()          {}
func (RenameRequested) isEvent()         {}
func (HitchSpawned) isEvent()            {}
func (HitchReady) isEvent()              {}
func (HitchLaunchFailed) isEvent()       {}
func (TurnStarted) isEvent()             {}
func (TurnBoundaryReached) isEvent()     {}
func (BlockedDetected) isEvent()         {}
func (BlockedCleared) isEvent()          {}
func (SendRequested) isEvent()           {}
func (TimedDeliveryReleased) isEvent()   {}
func (TimedDeliveriesCleared) isEvent()  {}
func (DeliverySucceeded) isEvent()       {}
func (DeliveryRetryRequested) isEvent()  {}
func (DeliveryDeferred) isEvent()        {}
func (DeliveryFailedEvent) isEvent()     {}
func (DeliveryUnverifiedEvent) isEvent() {}
func (CompactionRequested) isEvent()     {}
func (CompactionCompleted) isEvent()     {}
func (CompactionFailedEvent) isEvent()   {}
func (InterruptRequested) isEvent()      {}
func (InterruptSucceeded) isEvent()      {}
func (InterruptFailed) isEvent()         {}
func (DropRequested) isEvent()           {}
func (DropSucceeded) isEvent()           {}
func (DropFailed) isEvent()              {}
func (PaneVanished) isEvent()            {}
func (WedgeDetected) isEvent()           {}
func (WedgeCleared) isEvent()            {}
func (OperationTimedOut) isEvent()       {}
func (CurfewSet) isEvent()               {}
func (CurfewCleared) isEvent()           {}
func (TransitionRejected) isEvent()      {}
