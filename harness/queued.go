package harness

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"sort"
	"strings"
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
// turnID runs as its own turn once that turn finishes. Claude Code folds a
// prompt submitted mid-turn at a tool boundary into the running turn, removes
// it from the queue, and fires the submit hook with its text under the running
// turn's prompt id. A prompt still queued when the turn ends runs as its own
// turn under a new prompt id, so the finish hook of turnID must not close it;
// only the queue records show it is coming.
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
	var queue claudeQueue
	var ended bool
	var turnSeen time.Time
	prompts := map[string]bool{}
	for _, r := range records {
		if queue.replay(r) {
			if r.Operation == "dequeue" {
				ended = false
			}
			continue
		}
		switch {
		case r.Type == "system" && (r.Subtype == "stop_hook_summary" || r.Subtype == "turn_duration"):
			ended = true
		case r.Type == "user" && r.PromptID != "":
			if !prompts[r.PromptID] {
				prompts[r.PromptID] = true
				if ended {
					queue.queued, ended = 0, false
				}
			}
			if r.PromptID == turnID {
				turnSeen = r.Timestamp
			}
		}
	}
	// The dequeue can land before the finish hook reads the transcript.
	return queue.queued > 0 || (!turnSeen.IsZero() && queue.dequeued.After(turnSeen)), nil
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

// PromptTurn finds the prompt ids of the turn that ran the prompt opening
// with opening, that prompt's own id first. A prompt typed mid-turn waits in
// Claude Code's queue and runs as its own turn under a prompt id no hook
// announced, so only the transcript ties the prompt to that turn. Claude Code
// can dequeue several queued prompts into one turn, each with its own prompt
// id, and the transcript does not record which of them the turn's hooks
// carry. pending reports the prompt still waiting in the queue. left reports
// the prompt pulled out of the queue back into the composer, where it runs
// only if submitted again. A prompt the running turn absorbed, or one the
// transcript does not hold, has none of these.
func PromptTurn(invocation Invocation, transcript, opening string) (turns []string, pending, left bool, err error) {
	source, err := queuedTurnSource(invocation)
	if err != nil || source == "" || transcript == "" {
		return nil, false, false, err
	}
	records, err := claudeQueueRecords(transcript)
	if err != nil {
		return nil, false, false, err
	}
	for i, r := range records {
		switch {
		case r.Type == "user" && strings.HasPrefix(r.text(), opening):
			return dequeuedTogether(records, i), false, false, nil
		case r.Type == "queue-operation" && r.Operation == "enqueue" && strings.HasPrefix(r.text(), opening):
			pending, left = true, false
		case r.Type == "queue-operation" && r.Operation == "popAll" && strings.HasPrefix(r.text(), opening):
			// Claude Code writes popAll only when it moves queued prompts
			// into the composer for editing, one record per prompt.
			pending, left = false, true
		case r.Type == "queue-operation" && r.Operation == "remove" && strings.HasPrefix(r.text(), opening),
			r.Type == "queue-operation" && r.Operation == "popAll":
			pending = false
		}
	}
	return nil, pending, left, nil
}

// dequeuedTogether is the prompt ids of the queued user records written with
// records[i] after its dequeue and before the turn's first response, and placed
// in the same turn, records[i]'s own id first. Claude Code also writes prompts
// of different turns next to each other with no response between: a prompt
// typed after a local command, or after a prompt cancelled before any response,
// which keeps the cancelled prompt's turn position. A compaction summary
// repeats the turn position of the turn before it. So only queued records with
// no dequeue between them and records[i], in the same turn position, were
// dequeued together.
func dequeuedTogether(records []claudeQueueRecord, i int) []string {
	boundary := func(r claudeQueueRecord) bool {
		return r.Type == "assistant" || r.Type == "system" || r.Type == "queue-operation" && r.Operation == "dequeue"
	}
	start, end := i, i+1
	for start > 0 && !boundary(records[start-1]) {
		start--
	}
	for end < len(records) && !boundary(records[end]) {
		end++
	}
	ids := []string{records[i].PromptID}
	turn := records[i].TurnPosition
	for _, r := range records[start:end] {
		if r.Type == "user" && r.PromptSource == "queued" && turn != nil && r.TurnPosition != nil && r.TurnPosition.TurnIndex == turn.TurnIndex && !slices.Contains(ids, r.PromptID) {
			ids = append(ids, r.PromptID)
		}
	}
	return ids
}

// claudeQueue replays Claude Code's queue records: how many prompts wait in
// the queue, and when the last one left it to run.
type claudeQueue struct {
	queued   int
	dequeued time.Time
}

// replay applies r if it is a queue record and reports whether it was one.
func (q *claudeQueue) replay(r claudeQueueRecord) bool {
	if r.Type != "queue-operation" {
		return false
	}
	switch r.Operation {
	case "enqueue":
		q.queued++
	case "remove":
		q.queued = max(q.queued-1, 0)
	case "dequeue":
		q.queued, q.dequeued = max(q.queued-1, 0), r.Timestamp
	case "popAll":
		q.queued = 0
	}
	return true
}

type claudeQueueRecord struct {
	Type      string          `json:"type"`
	Subtype   string          `json:"subtype"`
	Operation string          `json:"operation"`
	PromptID  string          `json:"promptId"`
	Timestamp time.Time       `json:"timestamp"`
	Content   json.RawMessage `json:"content"`
	Message   json.RawMessage `json:"message"`
	// PromptSource is "queued" on a prompt Claude Code ran from its queue
	// and "typed" on one submitted at the composer.
	PromptSource string `json:"promptSource"`
	// Sidechain marks a subagent's record, Meta a record Claude Code wrote
	// itself, and CompactSummary a compaction's summary; none is a prompt
	// or a turn of the conversation.
	Sidechain      bool `json:"isSidechain"`
	Meta           bool `json:"isMeta"`
	CompactSummary bool `json:"isCompactSummary"`
	// TurnPosition places a prompt's user record in its turn; Claude Code
	// gives prompts it dequeues into one turn the same turn index.
	TurnPosition *struct {
		TurnIndex int `json:"turnIndex"`
	} `json:"turnPosition"`
}

// text is a queue operation's prompt or a user record's prompt text; any
// other shape reads as empty.
func (r claudeQueueRecord) text() string {
	content := r.Content
	if r.Type == "user" {
		var message struct {
			Content json.RawMessage `json:"content"`
		}
		if json.Unmarshal(r.Message, &message) != nil {
			return ""
		}
		content = message.Content
	}
	var text string
	if json.Unmarshal(content, &text) != nil {
		return ""
	}
	return text
}

func claudeQueueRecords(transcript string) ([]claudeQueueRecord, error) {
	file, err := OpenTranscript(transcript)
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
