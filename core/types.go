package core

import "time"

type TeamID string
type HitchID string
type EnvelopeID string
type CompactionID string
type AgentName string

type Team struct {
	ID     TeamID    `json:"id"`
	Name   string    `json:"name"`
	Curfew time.Time `json:"curfew,omitempty"`
}

type HitchStatus string

const (
	HitchStarting HitchStatus = "starting"
	HitchBooting  HitchStatus = "booting"
	HitchActive   HitchStatus = "active"
	HitchDropping HitchStatus = "dropping"
	HitchDropped  HitchStatus = "dropped"
	HitchFailed   HitchStatus = "failed"
)

type HitchActivity string

const (
	ActivityUnknown      HitchActivity = "unknown"
	ActivityIdle         HitchActivity = "idle"
	ActivityBusy         HitchActivity = "busy"
	ActivityDelivering   HitchActivity = "delivering"
	ActivityCompacting   HitchActivity = "compacting"
	ActivityInterrupting HitchActivity = "interrupting"
	ActivityBlocked      HitchActivity = "blocked"
	ActivityWedged       HitchActivity = "wedged"
)

type Hitch struct {
	ID                HitchID       `json:"id"`
	Name              AgentName     `json:"name"`
	Collar            string        `json:"collar"`
	Role              string        `json:"role,omitempty"`
	Directory         string        `json:"directory"`
	Status            HitchStatus   `json:"status,omitempty"`
	Activity          HitchActivity `json:"activity,omitempty"`
	Pane              string        `json:"pane,omitempty"`
	BootDeadline      time.Time     `json:"boot_deadline,omitempty"`
	DropDeadline      time.Time     `json:"drop_deadline,omitempty"`
	InterruptDeadline time.Time     `json:"interrupt_deadline,omitempty"`
	InterruptReason   string        `json:"interrupt_reason,omitempty"`
	PendingCompactID  CompactionID  `json:"pending_compact_id,omitempty"`
	BlockedEvidence   string        `json:"blocked_evidence,omitempty"`
	BlockedFrom       HitchActivity `json:"blocked_from,omitempty"`
	WedgeEvidence     string        `json:"wedge_evidence,omitempty"`
	PreviousStatus    HitchStatus   `json:"previous_status,omitempty"`
	PreviousActivity  HitchActivity `json:"previous_activity,omitempty"`
}

type Message struct {
	Text string `json:"text"`
}

type SenderKind string

const (
	SenderAgent        SenderKind = "agent"
	SenderSelfDeclared SenderKind = "self_declared"
)

// Sender is attributed, not authenticated. Agent senders are bound to the
// hitch Gangline observed; names from outside the team are explicitly marked.
type Sender struct {
	Kind    SenderKind `json:"kind"`
	Name    AgentName  `json:"name"`
	HitchID HitchID    `json:"hitch_id,omitempty"`
}

type Envelope struct {
	ID        EnvelopeID `json:"id"`
	From      Sender     `json:"from"`
	To        AgentName  `json:"to"`
	Message   Message    `json:"message"`
	CreatedAt time.Time  `json:"created_at"`
}

type DeliveryStatus string

const (
	DeliveryQueued     DeliveryStatus = "queued"
	DeliveryDelivering DeliveryStatus = "delivering"
	DeliveryDelivered  DeliveryStatus = "delivered"
	DeliveryFailed     DeliveryStatus = "failed"
	DeliveryUnverified DeliveryStatus = "unverified"
	DeliveryCancelled  DeliveryStatus = "cancelled"
)

type Delivery struct {
	MidTurn    bool           `json:"mid_turn,omitempty"`
	DuringTurn bool           `json:"during_turn,omitempty"`
	Envelope   Envelope       `json:"envelope"`
	Status     DeliveryStatus `json:"status"`
	Deadline   time.Time      `json:"deadline"`
	NotBefore  time.Time      `json:"not_before,omitempty"`
	Reason     string         `json:"reason,omitempty"`
}

type CompactionStatus string

const (
	CompactionQueued     CompactionStatus = "queued"
	CompactionRunning    CompactionStatus = "running"
	CompactionSucceeded  CompactionStatus = "succeeded"
	CompactionFailed     CompactionStatus = "failed"
	CompactionUnverified CompactionStatus = "unverified"
	CompactionCancelled  CompactionStatus = "cancelled"
)

type Compaction struct {
	ID       CompactionID     `json:"id"`
	HitchID  HitchID          `json:"hitch_id"`
	Resume   Message          `json:"resume"`
	Deadline time.Time        `json:"deadline"`
	Status   CompactionStatus `json:"status,omitempty"`
	Reason   string           `json:"reason,omitempty"`
}

type State struct {
	Team          Team                        `json:"team"`
	Hitches       map[HitchID]Hitch           `json:"hitches"`
	Deliveries    map[EnvelopeID]Delivery     `json:"deliveries"`
	DeliveryOrder []EnvelopeID                `json:"delivery_order"`
	Compactions   map[CompactionID]Compaction `json:"compactions"`
}

func NewState(team Team) State {
	return State{
		Team:        team,
		Hitches:     make(map[HitchID]Hitch),
		Deliveries:  make(map[EnvelopeID]Delivery),
		Compactions: make(map[CompactionID]Compaction),
	}
}
