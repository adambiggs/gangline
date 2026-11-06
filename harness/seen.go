package harness

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"
)

// SeenTokens reports which of tokens open an envelope in the input the
// harness gave its model, as recorded in transcript: a prompt that started
// a turn or one taken mid-turn. Queue records, tool output and recalled
// prompts do not count, because the model has not read them as input.
//
// It reads at most the last transcriptWindow bytes. since is the earliest
// record time it read, so a token absent from the result is unseen only for
// a message created at or after since.
func SeenTokens(invocation Invocation, transcript string, tokens []string) (seen map[string]bool, since time.Time, err error) {
	var input func([]byte) ([]string, time.Time, error)
	switch invocation.Name {
	case "claude-transcript":
		input = claudeInput
	case "codex-session-log":
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
	opener := regexp.MustCompile(`\[gang:[^\s\[\]#]+#(` + strings.Join(quoted, "|") + `)[\s\]]`)
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
			return seen, since, nil
		}
		if err != nil {
			return nil, time.Time{}, err
		}
		// Once the window's first time is known, only records that can
		// hold an envelope need decoding.
		if !since.IsZero() && !bytes.Contains(line, []byte("[gang:")) {
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

// claudeInput returns a Claude Code record's prompt text: a user prompt
// that is not a tool result, hook output or subagent turn, or a prompt the
// harness attached to a running turn.
func claudeInput(line []byte) ([]string, time.Time, error) {
	var r struct {
		Type        string          `json:"type"`
		Timestamp   time.Time       `json:"timestamp"`
		IsSidechain bool            `json:"isSidechain"`
		IsMeta      bool            `json:"isMeta"`
		Message     json.RawMessage `json:"message"`
		Attachment  struct {
			Type   string          `json:"type"`
			Prompt json.RawMessage `json:"prompt"`
		} `json:"attachment"`
	}
	if err := json.Unmarshal(line, &r); err != nil {
		return nil, time.Time{}, err
	}
	if r.IsSidechain || r.IsMeta {
		return nil, r.Timestamp, nil
	}
	switch {
	case r.Type == "user":
		var message struct {
			Content json.RawMessage `json:"content"`
		}
		if json.Unmarshal(r.Message, &message) != nil {
			return nil, r.Timestamp, nil
		}
		return promptTexts(message.Content, "text"), r.Timestamp, nil
	case r.Type == "attachment" && r.Attachment.Type == "queued_command":
		return promptTexts(r.Attachment.Prompt, "text"), r.Timestamp, nil
	}
	return nil, r.Timestamp, nil
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
	return promptTexts(r.Payload.Content, "input_text"), r.Timestamp, nil
}

// promptTexts reads content that is a string or a list of blocks, keeping
// only blocks of the given text type.
func promptTexts(content json.RawMessage, kind string) []string {
	var text string
	if json.Unmarshal(content, &text) == nil {
		return []string{text}
	}
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if json.Unmarshal(content, &blocks) != nil {
		return nil
	}
	var texts []string
	for _, b := range blocks {
		if b.Type == kind {
			texts = append(texts, b.Text)
		}
	}
	return texts
}
