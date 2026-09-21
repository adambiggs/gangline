package core

import "time"

//sumtype:decl
type Event interface {
	isEvent()
}

type HitchRequested struct {
	At    time.Time `json:"at"`
	Hitch Hitch     `json:"hitch"`
}

type HitchReady struct {
	At      time.Time `json:"at"`
	HitchID HitchID   `json:"hitch_id"`
	Pane    string    `json:"pane"`
}

type HitchLaunchFailed struct {
	At      time.Time `json:"at"`
	HitchID HitchID   `json:"hitch_id"`
	Reason  string    `json:"reason"`
}

type SendRequested struct {
	At       time.Time `json:"at"`
	Envelope Envelope  `json:"envelope"`
}

type DeliverySucceeded struct {
	At         time.Time  `json:"at"`
	EnvelopeID EnvelopeID `json:"envelope_id"`
}

type DeliveryFailedEvent struct {
	At         time.Time  `json:"at"`
	EnvelopeID EnvelopeID `json:"envelope_id"`
	Reason     string     `json:"reason"`
}

type DropRequested struct {
	At      time.Time `json:"at"`
	HitchID HitchID   `json:"hitch_id"`
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

type TransitionRejected struct {
	At     time.Time `json:"at"`
	Event  string    `json:"event"`
	Reason string    `json:"reason"`
}

func (HitchRequested) isEvent()      {}
func (HitchReady) isEvent()          {}
func (HitchLaunchFailed) isEvent()   {}
func (SendRequested) isEvent()       {}
func (DeliverySucceeded) isEvent()   {}
func (DeliveryFailedEvent) isEvent() {}
func (DropRequested) isEvent()       {}
func (DropSucceeded) isEvent()       {}
func (DropFailed) isEvent()          {}
func (TransitionRejected) isEvent()  {}
