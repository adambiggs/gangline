package core

import "time"

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

// Observation commits a native batch and its complete-record cursor together.
// Readings carry measurements; they do not drive delivery or turn transitions.
type Observation struct {
	At         time.Time `json:"at"`
	HitchID    HitchID   `json:"hitch_id"`
	Collar     string    `json:"collar"`
	SessionID  string    `json:"session_id,omitempty"`
	Transcript string    `json:"transcript,omitempty"`
	Offset     int64     `json:"offset,omitempty"`
	Readings   []Reading `json:"readings"`
}

func (Observation) isEvent() {}
