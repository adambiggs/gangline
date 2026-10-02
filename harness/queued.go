package harness

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"time"
)

// queuedTurnSource names where a hook-boundary collar reads the prompts its
// harness queued behind a running turn. Empty means it reads none.
func queuedTurnSource(invocation Invocation) (string, error) {
	switch value := invocation.Params["queued_turns"]; value {
	case "", "claude-transcript":
		return value, nil
	default:
		return "", fmt.Errorf("turn boundary queued_turns %q is not claude-transcript", value)
	}
}

// QueuedTurnPending reports whether a prompt the harness queued during turn
// turnID runs as its own turn once that turn finishes. Claude Code answers a
// prompt typed mid-turn with the running turn's submit hook and gives the
// queued turn none, so its finish hook is the only boundary the queued turn
// shows.
//
// The transcript's queue records are written out of time order, and a queued
// prompt can leave the queue unrecorded, so the queue is replayed by
// timestamp, and a new prompt after a turn ends with no dequeue empties it.
func QueuedTurnPending(invocation Invocation, transcript, turnID string) (bool, error) {
	source, err := queuedTurnSource(invocation)
	if err != nil || source == "" || transcript == "" {
		return false, err
	}
	records, err := claudeQueueRecords(transcript)
	if err != nil {
		return false, err
	}
	queued := 0
	var ended bool
	var dequeued, turnSeen time.Time
	prompts := map[string]bool{}
	for _, r := range records {
		switch {
		case r.Type == "queue-operation" && r.Operation == "enqueue":
			queued++
		case r.Type == "queue-operation" && r.Operation == "remove":
			queued = max(queued-1, 0)
		case r.Type == "queue-operation" && r.Operation == "dequeue":
			queued = max(queued-1, 0)
			ended, dequeued = false, r.Timestamp
		case r.Type == "queue-operation" && r.Operation == "popAll":
			queued = 0
		case r.Type == "system" && (r.Subtype == "stop_hook_summary" || r.Subtype == "turn_duration"):
			ended = true
		case r.Type == "user" && r.PromptID != "":
			if !prompts[r.PromptID] {
				prompts[r.PromptID] = true
				if ended {
					queued, ended = 0, false
				}
			}
			if r.PromptID == turnID {
				turnSeen = r.Timestamp
			}
		}
	}
	// The dequeue can land before the finish hook reads the transcript.
	return queued > 0 || (!turnSeen.IsZero() && dequeued.After(turnSeen)), nil
}

// TurnRanAfter reports whether the transcript records prompt later after
// prompt earlier. A queued prompt's turn fires no submit hook, so its finish
// hook carries a prompt id the witness never saw; only the transcript orders
// it against the turns before it. An id the transcript does not hold orders
// nothing.
func TurnRanAfter(invocation Invocation, transcript, earlier, later string) (bool, error) {
	source, err := queuedTurnSource(invocation)
	if err != nil || source == "" || transcript == "" {
		return false, err
	}
	records, err := claudeQueueRecords(transcript)
	if err != nil {
		return false, err
	}
	first := map[string]time.Time{}
	for _, r := range records {
		if _, seen := first[r.PromptID]; r.Type == "user" && r.PromptID != "" && !seen {
			first[r.PromptID] = r.Timestamp
		}
	}
	earlierAt, seen := first[earlier]
	return seen && first[later].After(earlierAt), nil
}

type claudeQueueRecord struct {
	Type      string    `json:"type"`
	Subtype   string    `json:"subtype"`
	Operation string    `json:"operation"`
	PromptID  string    `json:"promptId"`
	Timestamp time.Time `json:"timestamp"`
}

func claudeQueueRecords(transcript string) ([]claudeQueueRecord, error) {
	file, err := os.Open(transcript)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	size, err := file.Seek(0, io.SeekEnd)
	if err != nil {
		return nil, err
	}
	start, err := boundedTranscriptStart(file, 0, size)
	if err != nil {
		return nil, err
	}
	if _, err := file.Seek(start, io.SeekStart); err != nil {
		return nil, err
	}
	reader := bufio.NewReader(io.LimitReader(file, size-start))
	var records []claudeQueueRecord
	for {
		line, err := reader.ReadBytes('\n')
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		var record claudeQueueRecord
		if err := json.Unmarshal(line, &record); err != nil {
			return nil, fmt.Errorf("decode transcript queue record: %w", err)
		}
		if !record.Timestamp.IsZero() {
			records = append(records, record)
		}
	}
	sort.SliceStable(records, func(i, j int) bool { return records[i].Timestamp.Before(records[j].Timestamp) })
	return records, nil
}
