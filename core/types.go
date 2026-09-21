package core

import "time"

type TeamID string
type HitchID string
type EnvelopeID string
type AgentName string

type Team struct {
	ID   TeamID `json:"id"`
	Name string `json:"name"`
}

type HitchStatus string

const (
	HitchStarting HitchStatus = "starting"
	HitchActive   HitchStatus = "active"
	HitchDropping HitchStatus = "dropping"
	HitchDropped  HitchStatus = "dropped"
	HitchFailed   HitchStatus = "failed"
)

type Hitch struct {
	ID        HitchID     `json:"id"`
	Name      AgentName   `json:"name"`
	Collar    string      `json:"collar"`
	Role      string      `json:"role,omitempty"`
	Directory string      `json:"directory"`
	Status    HitchStatus `json:"status,omitempty"`
	Pane      string      `json:"pane,omitempty"`
}

type Message struct {
	Text string `json:"text"`
}

type Envelope struct {
	ID        EnvelopeID `json:"id"`
	From      AgentName  `json:"from"`
	To        AgentName  `json:"to"`
	Message   Message    `json:"message"`
	CreatedAt time.Time  `json:"created_at"`
}

type DeliveryStatus string

const (
	DeliveryPending   DeliveryStatus = "pending"
	DeliveryDelivered DeliveryStatus = "delivered"
	DeliveryFailed    DeliveryStatus = "failed"
)

type Delivery struct {
	Envelope Envelope       `json:"envelope"`
	Status   DeliveryStatus `json:"status"`
	Reason   string         `json:"reason,omitempty"`
}

type State struct {
	Team       Team                    `json:"team"`
	Hitches    map[HitchID]Hitch       `json:"hitches"`
	Deliveries map[EnvelopeID]Delivery `json:"deliveries"`
}

func NewState(team Team) State {
	return State{
		Team:       team,
		Hitches:    make(map[HitchID]Hitch),
		Deliveries: make(map[EnvelopeID]Delivery),
	}
}
