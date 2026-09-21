package core

import "time"

//sumtype:decl
type Effect interface {
	isEffect()
}

type SpawnHitch struct {
	Hitch Hitch
}

type AwaitBoot struct {
	HitchID  HitchID
	Pane     string
	Deadline time.Time
}

type DeliverEnvelope struct {
	Envelope Envelope
	Pane     string
	Deadline time.Time
}

type CompactHitch struct {
	Compaction Compaction
	Pane       string
}

type InterruptHitch struct {
	HitchID  HitchID
	Pane     string
	Reason   string
	Deadline time.Time
}

type KillHitch struct {
	HitchID  HitchID
	Pane     string
	Deadline time.Time
}

type RecordEvent struct {
	Event Event
}

func (SpawnHitch) isEffect()      {}
func (AwaitBoot) isEffect()       {}
func (DeliverEnvelope) isEffect() {}
func (CompactHitch) isEffect()    {}
func (InterruptHitch) isEffect()  {}
func (KillHitch) isEffect()       {}
func (RecordEvent) isEffect()     {}
