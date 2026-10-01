package main

import (
	"encoding/json"
	"io"
	"time"

	"github.com/adambiggs/gangline/core"
)

// The --json forms are the machine contract for observation commands. They
// carry exact values and report every availability condition as a field;
// the human forms may abbreviate and change layout.

type queueJSON struct {
	Messages []queueRow `json:"messages"`
}

type queueRow struct {
	ID        core.EnvelopeID `json:"id"`
	To        core.AgentName  `json:"to"`
	HitchID   core.HitchID    `json:"hitch_id"`
	From      core.Sender     `json:"from"`
	Kind      string          `json:"kind"`
	CreatedAt time.Time       `json:"created_at"`
	State     string          `json:"state"`
	DueAt     *time.Time      `json:"due_at,omitempty"`
	Reason    string          `json:"reason,omitempty"`
	Excerpt   string          `json:"excerpt"`
}

func writeJSON(w io.Writer, value any) error {
	return json.NewEncoder(w).Encode(value)
}
