package core

import "time"

// Event is an audit fact. Target identity travels with every agent event so
// filtering a log never needs to reconstruct state.
type Event struct {
	Source  string    `json:"source,omitempty"`
	Type    string    `json:"type"`
	At      time.Time `json:"at"`
	HitchID HitchID   `json:"hitch_id,omitempty"`
	Name    AgentName `json:"name,omitempty"`
	// To is the HOST:NAME address of a message forwarded to a peer host.
	To          string           `json:"to,omitempty"`
	Pane        string           `json:"pane,omitempty"`
	ID          string           `json:"id,omitempty"`
	Reason      string           `json:"reason,omitempty"`
	Status      string           `json:"status,omitempty"`
	Activity    Activity         `json:"activity,omitempty"`
	Deadline    time.Time        `json:"deadline,omitzero"`
	Envelope    *Envelope        `json:"envelope,omitempty"`
	Compaction  *Compaction      `json:"compaction,omitempty"`
	Readings    []Reading        `json:"readings,omitempty"`
	NativeEvent string           `json:"native_event,omitempty"`
	Fingerprint string           `json:"fingerprint,omitempty"`
	Basis       *ActivityBasis   `json:"basis,omitempty"`
	Binding     *Binding         `json:"binding,omitempty"`
	Actor       *CredentialActor `json:"actor,omitempty"`
	Command     string           `json:"command,omitempty"`
}

// CredentialActor identifies the verified caller or the operator's shell.
type CredentialActor struct {
	Kind    string    `json:"kind"`
	Name    AgentName `json:"name,omitempty"`
	HitchID HitchID   `json:"hitch_id,omitempty"`
}

// CredentialOrigin records the command that created or changed a registration.
type CredentialOrigin struct {
	At      time.Time       `json:"at"`
	Actor   CredentialActor `json:"actor"`
	Command string          `json:"command"`
}

// Binding records the native session a hitch is bound to, with the collar
// that launched it and the hitch that hitched it, so a session can be joined
// to its hitch after the agent's state is gone.
type Binding struct {
	SessionID         string  `json:"session_id"`
	PreviousSessionID string  `json:"previous_session_id,omitempty"`
	Collar            string  `json:"collar"`
	HitchedBy         HitchID `json:"hitched_by,omitempty"`
}

// ActivityBasis records what an activity reading was derived from, without
// screen text: what the collar read on the pane, the rule that set the
// reading, and the compaction record's status when there is one.
type ActivityBasis struct {
	Screen     string `json:"screen"`
	Rule       string `json:"rule"`
	Compaction string `json:"compaction,omitempty"`
}

// Effect reports a transition rejection to the command.
type Effect struct {
	Kind   string
	Reason string
}
