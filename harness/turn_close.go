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
// id the transcript does not hold leaves the turn open. Subagent, meta and
// compaction-summary records are not turns of the conversation.
func TurnClosed(invocation Invocation, transcript, turnID string) (TurnClose, bool, error) {
	source, err := turnCloseSource(invocation)
	if err != nil || source == "" || transcript == "" || turnID == "" {
		return TurnClose{}, false, err
	}
	records, err := claudeQueueRecords(transcript)
	if err != nil {
		return TurnClose{}, false, err
	}
	// The whole window is replayed: a prompt queued before turnID began can
	// still run after it. turnID's first record starts a turn, so an end
	// recorded after the last prompt to start ends turnID or a later turn.
	var queue claudeQueue
	var seen, ended, interrupted bool
	var closed TurnClose
	var started time.Time
	// claims counts dequeue records that no prompt run from the queue has
	// matched yet. A queued prompt with none to match left the queue
	// unrecorded, so it leaves the count itself.
	claims := 0
	prompts := map[string]bool{}
	for _, r := range records {
		if r.Sidechain {
			continue
		}
		if queue.replay(r) {
			if r.Operation == "dequeue" {
				claims++
			}
			continue
		}
		switch {
		case r.Type == "system" && r.Subtype == "turn_duration":
			ended, closed = true, TurnClose{At: r.Timestamp, Interrupted: interrupted}
		case r.Type == "user":
			interrupted = r.userInterrupt()
			if r.PromptID == "" || r.Meta || r.CompactSummary || prompts[r.PromptID] {
				continue
			}
			prompts[r.PromptID], started = true, r.Timestamp
			seen = seen || r.PromptID == turnID
			switch {
			case r.PromptSource == "queued" && claims > 0:
				claims--
			case r.PromptSource == "queued":
				queue.queued = max(queue.queued-1, 0)
			case ended && (r.PromptSource == "" || r.PromptSource == "typed"):
				// A prompt typed after an end shows the queue was empty.
				queue.queued = 0
			}
			ended = false
		}
	}
	if !seen {
		return TurnClose{}, false, nil
	}
	// The next turn's dequeue can be written before the end of this one.
	if !ended || queue.queued > 0 || queue.dequeued.After(started) {
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
