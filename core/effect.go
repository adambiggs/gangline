package core

//sumtype:decl
type Effect interface {
	isEffect()
}

type SpawnHitch struct {
	Hitch Hitch
}

type DeliverEnvelope struct {
	Envelope Envelope
	Pane     string
}

type KillHitch struct {
	HitchID HitchID
	Pane    string
}

type RecordEvent struct {
	Event Event
}

func (SpawnHitch) isEffect()      {}
func (DeliverEnvelope) isEffect() {}
func (KillHitch) isEffect()       {}
func (RecordEvent) isEffect()     {}
