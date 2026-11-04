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
	Messages  []queueRow `json:"messages"`
	Total     int        `json:"total"`
	Truncated bool       `json:"truncated"`
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
	Total             int         `json:"total"`
	Truncated         bool        `json:"truncated"`
}

// partialRosterJSON is emitted only when the caller explicitly accepts partial
// coverage. The strict roster contract keeps its original shape.
type partialRosterJSON struct {
	rosterJSON
	Partial bool          `json:"partial"`
	Errors  []rosterError `json:"errors"`
}

type rosterError struct {
	Kind    string          `json:"kind"`
	HitchID *core.HitchID   `json:"hitch_id"`
	Name    *core.AgentName `json:"name"`
	Stage   string          `json:"stage"`
}

type agentJSON struct {
	Kind             string            `json:"kind"`
	InputOutage      *core.InputOutage `json:"input_outage,omitempty"`
	Name             core.AgentName    `json:"name"`
	HitchID          core.HitchID      `json:"hitch_id"`
	Status           core.Status       `json:"status"`
	Activity         core.Activity     `json:"activity"`
	Collar           string            `json:"collar"`
	ConfiguredModel  *string           `json:"configured_model"`
	ConfiguredEffort *string           `json:"configured_effort"`
	ObservedModel    *string           `json:"observed_model"`
	ObservedEffort   *string           `json:"observed_effort"`
	Pane             string            `json:"pane"`
	ProcessAvailable bool              `json:"process_available"`
	Evidence         string            `json:"evidence"`
	Compaction       *compactionJSON   `json:"compaction,omitempty"`
	// BackgroundTasks qualifies idle with a stored background report's count,
	// not a verified current task count. It never marks the agent busy.
	BackgroundTasks int `json:"background_tasks,omitempty"`
	// BackgroundSince is when the background report was received.
	BackgroundSince *time.Time `json:"background_since,omitempty"`
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
