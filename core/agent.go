package core

import "time"

type HitchID string
type EnvelopeID string
type AgentName string

type Team struct {
	Curfew time.Time `json:"curfew,omitzero"`
}

type Status string

const (
	Starting Status = "starting"
	Booting  Status = "booting"
	Active   Status = "active"
	Dropping Status = "dropping"
	Failed   Status = "failed"
)

type Activity string

const (
	Unknown      Activity = "unknown"
	Idle         Activity = "idle"
	Busy         Activity = "busy"
	Compacting   Activity = "compacting"
	Interrupting Activity = "interrupting"
	Blocked      Activity = "blocked"
	Wedged       Activity = "wedged"
)

// Agent contains only the facts needed to operate one harness.
type Agent struct {
	ID                HitchID           `json:"id"`
	Name              AgentName         `json:"name"`
	Collar            string            `json:"collar"`
	Role              string            `json:"role,omitempty"`
	Directory         string            `json:"directory"`
	Pane              string            `json:"pane,omitempty"`
	Status            Status            `json:"status"`
	Activity          Activity          `json:"activity"`
	CreatedAt         time.Time         `json:"created_at"`
	ChangedAt         time.Time         `json:"changed_at"`
	BootDeadline      time.Time         `json:"boot_deadline,omitzero"`
	InterruptDeadline time.Time         `json:"interrupt_deadline,omitzero"`
	DropDeadline      time.Time         `json:"drop_deadline,omitzero"`
	Evidence          string            `json:"evidence,omitempty"`
	ScreenFingerprint string            `json:"screen_fingerprint,omitempty"`
	ScreenSince       time.Time         `json:"screen_since,omitzero"`
	Input             *InputIntent      `json:"input,omitempty"`
	Compaction        *Compaction       `json:"compaction,omitempty"`
	Capacity          Capacity          `json:"capacity,omitzero"`
	Native            NativeState       `json:"native,omitzero"`
	ContextBands      ContextBandState  `json:"context_bands,omitzero"`
	Process           ProcessIdentity   `json:"process,omitzero"`
	Teardown          []ProcessIdentity `json:"teardown,omitempty"`
	RenameFrom        AgentName         `json:"rename_from,omitempty"`
	RenameTo          AgentName         `json:"rename_to,omitempty"`
	LastDelivered     EnvelopeID        `json:"last_delivered,omitempty"`
	LastFailed        EnvelopeID        `json:"last_failed,omitempty"`
	Cleanup           *ResultRef        `json:"cleanup,omitempty"`
}

// ContextBandState keeps the last known reading and publication intents under
// the agent lock. Unknown readings never reset a crossing.
type ContextBandState struct {
	Model       string            `json:"model,omitempty"`
	Percent     float64           `json:"percent,omitempty"`
	CompactedAt time.Time         `json:"compacted_at,omitzero"`
	Sequence    uint64            `json:"sequence,omitempty"`
	Pending     []ContextBandNote `json:"pending,omitempty"`
}

type ContextBandNote struct {
	Band     string   `json:"band"`
	Reading  Reading  `json:"reading"`
	Envelope Envelope `json:"envelope"`
}

type ProcessIdentity struct {
	PID      int    `json:"pid"`
	Started  string `json:"started"`
	Version  uint32 `json:"version,omitempty"`
	UniqueID uint64 `json:"unique_id,omitempty"`
	BootID   string `json:"boot_id"`
}

type InputIntent struct {
	ID   string    `json:"id"`
	Kind string    `json:"kind"`
	At   time.Time `json:"at"`
}

type ResultRef struct {
	ID        EnvelopeID `json:"id"`
	Directory string     `json:"directory"`
}

type Message struct {
	Text string `json:"text"`
}
type Sender struct {
	Kind    string    `json:"kind"`
	Name    AgentName `json:"name"`
	HitchID HitchID   `json:"hitch_id,omitempty"`
}

const (
	SenderAgent        = "agent"
	SenderSelfDeclared = "self_declared"
	SenderGangline     = "gangline"
)

func (s Sender) SameIdentity(other Sender) bool {
	if s.Kind != other.Kind {
		return false
	}
	if s.Kind == SenderAgent {
		return s.HitchID != "" && s.HitchID == other.HitchID
	}
	return s.Name == other.Name
}

type Envelope struct {
	ID        EnvelopeID `json:"id"`
	From      Sender     `json:"from"`
	To        AgentName  `json:"to"`
	Recipient HitchID    `json:"recipient"`
	Message   Message    `json:"message"`
	Purpose   string     `json:"purpose,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	NotBefore time.Time  `json:"not_before,omitzero"`
	Outcome   string     `json:"outcome,omitempty"`
	Reason    string     `json:"reason,omitempty"`
}

type Compaction struct {
	ID            string    `json:"id"`
	Resume        Message   `json:"resume"`
	StartedAt     time.Time `json:"started_at"`
	Deadline      time.Time `json:"deadline"`
	Status        string    `json:"status"`
	Continuation  bool      `json:"continuation,omitempty"`
	CompletedAt   time.Time `json:"completed_at,omitzero"`
	RefusalBefore int       `json:"refusal_before,omitempty"`
	Reason        string    `json:"reason,omitempty"`
}

type Capacity struct {
	Fingerprint string    `json:"fingerprint,omitempty"`
	Evidence    string    `json:"evidence,omitempty"`
	Deadline    time.Time `json:"deadline,omitzero"`
	NextAt      time.Time `json:"next_at,omitzero"`
	Attempts    int       `json:"attempts,omitempty"`
	Submitted   bool      `json:"submitted,omitempty"`
}

type NativeState struct {
	SessionID   string    `json:"session_id,omitempty"`
	TurnID      string    `json:"turn_id,omitempty"`
	Transcript  string    `json:"transcript,omitempty"`
	Offset      int64     `json:"offset,omitempty"`
	SubmittedAt time.Time `json:"submitted_at,omitzero"`
	CompactedAt time.Time `json:"compacted_at,omitzero"`
	Context     Reading   `json:"context,omitzero"`
	Limits      Reading   `json:"limits,omitzero"`
	Model       string    `json:"model,omitempty"`
}

type Reading struct {
	Kind        string        `json:"kind"`
	Source      string        `json:"source"`
	NativeEvent string        `json:"native_event,omitempty"`
	At          *time.Time    `json:"at,omitempty"`
	Status      string        `json:"status"`
	Reason      string        `json:"reason,omitempty"`
	Model       string        `json:"model,omitempty"`
	Used        *int64        `json:"used,omitempty"`
	Limit       *int64        `json:"limit,omitempty"`
	Percent     *float64      `json:"percent,omitempty"`
	Limits      []LimitWindow `json:"limits,omitempty"`
}
type LimitWindow struct {
	Label       string  `json:"label"`
	UsedPercent float64 `json:"used_percent"`
	ResetAt     int64   `json:"reset_at"`
}

func RetryDelay(attempt int) time.Duration {
	delay := 100 * time.Millisecond
	for attempt > 0 && delay < 30*time.Second {
		delay = min(2*delay, 30*time.Second)
		attempt--
	}
	return delay
}
