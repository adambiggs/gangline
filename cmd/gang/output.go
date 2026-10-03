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

type rosterJSON struct {
	WatchdogAvailable bool        `json:"watchdog_available"`
	Agents            []agentJSON `json:"agents"`
}

type agentJSON struct {
	Name             core.AgentName  `json:"name"`
	HitchID          core.HitchID    `json:"hitch_id"`
	Status           core.Status     `json:"status"`
	Activity         core.Activity   `json:"activity"`
	Collar           string          `json:"collar"`
	Pane             string          `json:"pane"`
	ProcessAvailable bool            `json:"process_available"`
	Evidence         string          `json:"evidence"`
	Compaction       *compactionJSON `json:"compaction,omitempty"`
	// BackgroundTasks qualifies idle with the native background tasks the
	// last turn left pending. It never marks the agent busy.
	BackgroundTasks int `json:"background_tasks,omitempty"`
}

type compactionJSON struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Reason string `json:"reason"`
}

// contextJSON is an observed reading, or an unknown one with its reason and
// null counts.
type contextJSON struct {
	Name    core.AgentName `json:"name"`
	Status  string         `json:"status"`
	Reason  string         `json:"reason,omitempty"`
	Model   string         `json:"model"`
	Used    *int64         `json:"used"`
	Limit   *int64         `json:"limit"`
	Percent *float64       `json:"percent"`
	Band    *string        `json:"band"`
	Source  string         `json:"source"`
}

func writeJSON(w io.Writer, value any) error {
	return json.NewEncoder(w).Encode(value)
}
