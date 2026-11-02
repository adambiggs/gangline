package harness

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"sort"
	"strings"
	"time"
)

// ClaudeTurn is a selected retained turn, not a live lifecycle observation.
// Output remains private tool-result data for the caller's reference check.
type ClaudeTurn struct {
	Offset       int64
	Complete     bool
	Model        string
	MessageID    string
	RequestID    string
	CallRecord   string
	ResultRecord string
	ResultOffset int64
	Output       string
	Witness      bool
}

type claudeEvidenceRecord struct {
	Type      string    `json:"type"`
	Subtype   string    `json:"subtype"`
	Session   string    `json:"sessionId"`
	Prompt    string    `json:"promptId"`
	UUID      string    `json:"uuid"`
	Request   string    `json:"requestId"`
	Sidechain bool      `json:"isSidechain"`
	Meta      bool      `json:"isMeta"`
	Summary   bool      `json:"isCompactSummary"`
	APIError  bool      `json:"isApiErrorMessage"`
	At        time.Time `json:"timestamp"`
	Message   struct {
		ID      string          `json:"id"`
		Role    string          `json:"role"`
		Model   string          `json:"model"`
		Stop    string          `json:"stop_reason"`
		Content json.RawMessage `json:"content"`
	} `json:"message"`
	end int64
}

type claudeEvidenceBlock struct {
	Type    string          `json:"type"`
	ID      string          `json:"id"`
	Call    string          `json:"tool_use_id"`
	Content json.RawMessage `json:"content"`
	Error   bool            `json:"is_error"`
	Text    string          `json:"text"`
}

// ReadClaudeTurn reuses the native identity and bounded transcript readers.
// It requires a promptId, a matching tool_use/tool_result, and end_turn followed
// by turn_duration. EOF, status-line samples and stop-hook summaries cannot
// establish successful settlement. Sidechains and compaction summaries cannot
// supply the selected main-session prompt or tool witness.
func ReadClaudeTurn(input io.ReadSeeker, session, prompt, call string) (ClaudeTurn, error) {
	var result ClaudeTurn
	if session == "" || prompt == "" {
		return result, errors.New("native run identity missing")
	}
	if _, err := input.Seek(0, io.SeekStart); err != nil {
		return result, err
	}
	id, err := claudeTranscriptIdentity(input)
	if err != nil || id != session {
		return result, errors.New("native session identity mismatch")
	}
	size, err := input.Seek(0, io.SeekEnd)
	if err != nil {
		return result, err
	}
	start, err := boundedTranscriptStart(input, 0, size)
	if err != nil {
		return result, err
	}
	if _, err = input.Seek(start, io.SeekStart); err != nil {
		return result, err
	}
	result.Offset = start
	reader := bufio.NewReader(io.LimitReader(input, size-start))
	var records []claudeEvidenceRecord
	seen := map[string][32]byte{}
	for {
		line, err := reader.ReadBytes('\n')
		if err == io.EOF {
			break
		}
		if err != nil {
			return result, err
		}
		result.Offset += int64(len(line))
		var r claudeEvidenceRecord
		if json.Unmarshal(line, &r) != nil {
			return result, errors.New("invalid native transcript record")
		}
		if r.Sidechain {
			continue
		}
		if r.Session != "" && r.Session != session {
			return result, errors.New("native session identity mismatch")
		}
		if r.Type != "user" && r.Type != "assistant" && !(r.Type == "system" && r.Subtype == "turn_duration") {
			continue
		}
		if r.Session == "" || r.At.IsZero() {
			return result, errors.New("native record identity or timestamp missing")
		}
		if r.UUID != "" {
			hash := sha256.Sum256(bytes.TrimSpace(line))
			if previous, ok := seen[r.UUID]; ok {
				if previous != hash {
					return result, errors.New("conflicting native record identity")
				}
				continue
			}
			seen[r.UUID] = hash
		}
		r.end = result.Offset
		records = append(records, r)
	}
	// Claude may append queued prompt records out of timestamp order.
	sort.SliceStable(records, func(i, j int) bool { return records[i].At.Before(records[j].At) })
	active, ended, haveCall := false, false, false
	for _, r := range records {
		if r.Meta || r.Summary {
			continue
		}
		var blocks []claudeEvidenceBlock
		_ = json.Unmarshal(r.Message.Content, &blocks)
		toolResult := false
		for _, b := range blocks {
			if b.Type == "tool_result" {
				toolResult = true
			}
		}
		if r.Type == "user" && !toolResult && r.Prompt != "" {
			_, hasPrompt := claudeEvidenceText(r.Message.Content)
			active = r.Prompt == prompt && r.Message.Role == "user" && hasPrompt
			ended, haveCall = false, false
			if active {
				result = ClaudeTurn{Offset: result.Offset}
			}
		}
		if !active || (r.Prompt != "" && r.Prompt != prompt) {
			continue
		}
		switch r.Type {
		case "assistant":
			if r.Message.Role != "assistant" || r.APIError {
				return result, errors.New("selected native turn failed or has invalid role")
			}
			if r.Message.Model != "" {
				result.Model = r.Message.Model
			}
			ended = r.Message.Stop == "end_turn"
			for _, b := range blocks {
				if b.Type != "tool_use" || call == "" || b.ID != call {
					continue
				}
				if haveCall {
					return result, errors.New("ambiguous native tool call")
				}
				haveCall = true
				result.CallRecord = r.UUID
				result.MessageID = r.Message.ID
				result.RequestID = r.Request
			}
		case "user":
			for _, b := range blocks {
				if b.Type != "tool_result" || call == "" || b.Call != call {
					continue
				}
				ended = false
				if r.Message.Role != "user" {
					return result, errors.New("invalid native tool-result role")
				}
				if !haveCall {
					continue
				}
				if b.Error {
					return result, errors.New("selected native tool result failed")
				}
				if result.Witness {
					return result, errors.New("ambiguous native tool result")
				}
				result.Output, result.Witness = claudeEvidenceText(b.Content)
				result.ResultRecord = r.UUID
				result.ResultOffset = r.end
			}
		case "system":
			if r.Subtype == "turn_duration" && ended {
				result.Complete = true
				active = false
			}
		}
	}
	return result, nil
}

func claudeEvidenceText(data json.RawMessage) (string, bool) {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || bytes.Equal(data, []byte("null")) {
		return "", false
	}
	var text string
	if json.Unmarshal(data, &text) == nil {
		return text, true
	}
	var blocks []claudeEvidenceBlock
	if json.Unmarshal(data, &blocks) != nil {
		return "", false
	}
	var parts []string
	for _, b := range blocks {
		if b.Type == "text" {
			parts = append(parts, b.Text)
		}
	}
	return strings.Join(parts, "\n"), len(parts) > 0
}
