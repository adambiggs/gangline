package harness

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// turnCloseSource names where a hook-boundary collar reads that its harness
// ended a turn whose finish hook may never run. Empty means it reads none.
func turnCloseSource(invocation Invocation) (string, error) {
	switch value := invocation.Params["turn_close"]; value {
	case "", "claude-transcript":
		return value, nil
	default:
		return "", fmt.Errorf("turn boundary turn_close %q is not claude-transcript", value)
	}
}

// TurnClose is the transcript's record that a turn ended.
type TurnClose struct {
	// At is when the last turn ended.
	At time.Time
	// Interrupted reports that the user interrupted the last turn.
	Interrupted bool
}

// TurnClosed reports whether the transcript records that turn turnID, and
// every turn that ran from the queue after it, ended. Claude Code writes
// system/turn_duration at the end of every turn, including one that Escape
// ends, which fires no hook; a user record reading "[Request interrupted by
// user" just before it marks the turn interrupted. A prompt still queued, a
// dequeue for the next turn, a prompt recorded after the last end, or a turn
// id the transcript does not hold leaves the turn open.
func TurnClosed(invocation Invocation, transcript, turnID string) (TurnClose, bool, error) {
	source, err := turnCloseSource(invocation)
	if err != nil || source == "" || transcript == "" || turnID == "" {
		return TurnClose{}, false, err
	}
	records, err := claudeQueueRecords(transcript)
	if err != nil {
		return TurnClose{}, false, err
	}
	start := -1
	for i, r := range records {
		if r.Type == "user" && r.PromptID == turnID {
			start = i
			break
		}
	}
	if start < 0 {
		return TurnClose{}, false, nil
	}
	queued := 0
	var ended, interrupted bool
	var closed TurnClose
	var dequeued, started time.Time
	prompts := map[string]bool{}
	for _, r := range records[start:] {
		switch {
		case r.Type == "queue-operation" && r.Operation == "enqueue":
			queued++
		case r.Type == "queue-operation" && r.Operation == "remove":
			queued = max(queued-1, 0)
		case r.Type == "queue-operation" && r.Operation == "dequeue":
			queued, dequeued = max(queued-1, 0), r.Timestamp
		case r.Type == "queue-operation" && r.Operation == "popAll":
			queued = 0
		case r.Type == "system" && r.Subtype == "turn_duration":
			ended, closed = true, TurnClose{At: r.Timestamp, Interrupted: interrupted}
		case r.Type == "user":
			interrupted = r.userInterrupt()
			if r.PromptID != "" && !prompts[r.PromptID] {
				prompts[r.PromptID], started = true, r.Timestamp
				// A queued prompt can leave the queue unrecorded; a new
				// turn after an end shows the queue it ran from is empty.
				if ended {
					queued, ended = 0, false
				}
			}
		}
	}
	// The next turn's dequeue can be written before the end of this one.
	if !ended || queued > 0 || dequeued.After(started) {
		return TurnClose{}, false, nil
	}
	return closed, true, nil
}

// userInterrupt reports that a user record is Claude Code's note that the
// user interrupted the turn, written as text or as a text block.
func (r claudeQueueRecord) userInterrupt() bool {
	const note = "[Request interrupted by user"
	if strings.HasPrefix(r.text(), note) {
		return true
	}
	var message struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if json.Unmarshal(r.Message, &message) != nil {
		return false
	}
	for _, block := range message.Content {
		if block.Type == "text" && strings.HasPrefix(block.Text, note) {
			return true
		}
	}
	return false
}
