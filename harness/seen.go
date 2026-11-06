package harness

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"
)

// SeenTokens reports which of tokens open an envelope in the input the
// harness gave its model, as recorded in session's transcript: a prompt that
// started a turn or one taken mid-turn. Queue records, tool output, recalled
// prompts and compaction summaries do not count, because the model has not
// read them as input.
//
// It reads at most the last transcriptWindow bytes. since is the time of the
// first timed record it read, which is no later than when it was written,
// so a token absent from the result is unseen for a message created at or
// after since. A window with no timed record cannot bound that, and is an
// error.
func SeenTokens(invocation Invocation, transcript, session string, tokens []string) (seen map[string]bool, since time.Time, err error) {
	if session == "" {
		return nil, time.Time{}, errors.New("no native session is bound")
	}
	var input func([]byte) ([]string, time.Time, error)
	switch invocation.Name {
	case "claude-transcript":
		input = claudeInput(session)
	case "codex-session-log":
		// A misbound log is resolved to the session's own, as context
		// reading does.
		if transcript, err = ResolveTranscript(invocation, transcript, session); err != nil {
			return nil, time.Time{}, err
		}
		input = codexInput
	default:
		return nil, time.Time{}, fmt.Errorf("unknown seen primitive %q", invocation.Name)
	}
	if len(tokens) == 0 {
		return map[string]bool{}, time.Time{}, nil
	}
	quoted := make([]string, 0, len(tokens))
	for _, token := range tokens {
		quoted = append(quoted, regexp.QuoteMeta(token))
	}
	// Both tag forms: an agent's [gang:NAME#TOKEN and Gangline's own
	// [gang#TOKEN, whose startup sections follow the token with -SECTION.
	opener := regexp.MustCompile(`\[gang(?::[^\s\[\]#]+)?#(` + strings.Join(quoted, "|") + `)[\s\]-]`)
	file, err := OpenTranscript(transcript)
	if err != nil {
		return nil, time.Time{}, err
	}
	defer file.Close()
	size, err := file.Seek(0, io.SeekEnd)
	if err != nil {
		return nil, time.Time{}, err
	}
	start, err := boundedTranscriptStart(file, 0, size)
	if err != nil {
		return nil, time.Time{}, err
	}
	if _, err := file.Seek(start, io.SeekStart); err != nil {
		return nil, time.Time{}, err
	}
	seen = map[string]bool{}
	reader := bufio.NewReader(io.LimitReader(file, size-start))
	for {
		line, err := reader.ReadBytes('\n')
		if err == io.EOF {
			// A record still being written is not yet evidence.
			if since.IsZero() {
				return nil, time.Time{}, errors.New("transcript window holds no timed record")
			}
			return seen, since, nil
		}
		if err != nil {
			return nil, time.Time{}, err
		}
		// Once the window's first time is known, only records that can
		// hold an envelope need decoding.
		if !since.IsZero() && !bytes.Contains(line, []byte("[gang:")) && !bytes.Contains(line, []byte("[gang#")) {
			continue
		}
		texts, at, err := input(line)
		if err != nil {
			return nil, time.Time{}, fmt.Errorf("decode transcript record: %w", err)
		}
		if since.IsZero() && !at.IsZero() {
			since = at
		}
		for _, text := range texts {
			for _, m := range opener.FindAllStringSubmatch(text, -1) {
				seen[m[1]] = true
			}
		}
	}
}

// claudeInput returns a reader of a Claude Code record's prompt text: a user
// prompt that is not a tool result, hook output, compaction summary or
// subagent turn, or a prompt the harness attached to a running turn. A main
// record naming another session is refused, as turn reading refuses it.
func claudeInput(session string) func([]byte) ([]string, time.Time, error) {
	return func(line []byte) ([]string, time.Time, error) {
		var r struct {
			claudeEvidenceRecord
			Attachment struct {
				Type   string          `json:"type"`
				Prompt json.RawMessage `json:"prompt"`
			} `json:"attachment"`
		}
		if err := json.Unmarshal(line, &r); err != nil {
			return nil, time.Time{}, err
		}
		if r.Sidechain {
			return nil, r.At, nil
		}
		if r.Session != "" && r.Session != session {
			return nil, time.Time{}, errors.New("native session identity mismatch")
		}
		if r.Meta || r.Summary {
			return nil, r.At, nil
		}
		var content json.RawMessage
		switch {
		case r.Type == "user":
			content = r.Message.Content
		case r.Type == "attachment" && r.Attachment.Type == "queued_command":
			content = r.Attachment.Prompt
		default:
			return nil, r.At, nil
		}
		if text, ok := claudeEvidenceText(content); ok {
			return []string{text}, r.At, nil
		}
		return nil, r.At, nil
	}
}

// codexInput returns a Codex session-log record's user message text. Codex
// writes it when the input enters the conversation: when its turn starts,
// or, for input taken mid-turn, after the step that was running.
func codexInput(line []byte) ([]string, time.Time, error) {
	var r struct {
		Type      string    `json:"type"`
		Timestamp time.Time `json:"timestamp"`
		Payload   struct {
			Type    string          `json:"type"`
			Role    string          `json:"role"`
			Content json.RawMessage `json:"content"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(line, &r); err != nil {
		return nil, time.Time{}, err
	}
	if r.Type != "response_item" || r.Payload.Type != "message" || r.Payload.Role != "user" {
		return nil, r.Timestamp, nil
	}
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if json.Unmarshal(r.Payload.Content, &blocks) != nil {
		return nil, r.Timestamp, nil
	}
	var texts []string
	for _, b := range blocks {
		if b.Type == "input_text" {
			texts = append(texts, b.Text)
		}
	}
	return texts, r.Timestamp, nil
}
