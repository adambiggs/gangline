package harness

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// sessionMoveSource names where a hook-boundary collar reads that its harness
// moved a conversation to a new native session. Empty means it reads none, so
// no session change is followed.
func sessionMoveSource(invocation Invocation) (string, error) {
	switch value := invocation.Params["session_moves"]; value {
	case "", "claude-transcript":
		return value, nil
	default:
		return "", fmt.Errorf("turn boundary session_moves %q is not claude-transcript", value)
	}
}

// maxSessionMoves bounds how many recorded moves SessionMoved follows from one
// session to reach another, so a cycle of records ends.
const maxSessionMoves = 8

// SessionMoved reports whether the harness's own records prove that session
// next, whose transcript is nextTranscript, continues the conversation of
// session from, whose transcript is fromTranscript. Claude Code moves a
// conversation to a background session under a new id, writes the new
// session's transcript beside the old one, and ends the old transcript with a
// continued-in record naming the new id. A prompt or reply recorded after
// that record means the old session went on by itself, so it proves no move.
// A session reached by several moves is followed record by record.
func SessionMoved(invocation Invocation, fromTranscript, from, next, nextTranscript string) (bool, error) {
	source, err := sessionMoveSource(invocation)
	if err != nil || source == "" || fromTranscript == "" || from == "" || next == "" || next == from {
		return false, err
	}
	dir := filepath.Dir(fromTranscript)
	if nextTranscript != filepath.Join(dir, next+".jsonl") {
		return false, nil
	}
	transcript, session := fromTranscript, from
	for range maxSessionMoves {
		moved, err := claudeContinuedIn(transcript, session)
		if errors.Is(err, os.ErrNotExist) {
			// A move to a session that never wrote a transcript proves no
			// later move.
			return false, nil
		}
		if err != nil || moved == "" {
			return false, err
		}
		if moved == next {
			return true, nil
		}
		transcript, session = filepath.Join(dir, moved+".jsonl"), moved
	}
	return false, nil
}

// claudeContinuedIn is the session id the last continued-in record of session's
// transcript names, or empty when the transcript records a prompt or reply
// after it, or has none.
func claudeContinuedIn(transcript, session string) (string, error) {
	file, err := os.Open(transcript)
	if err != nil {
		return "", err
	}
	defer file.Close()
	size, err := file.Seek(0, io.SeekEnd)
	if err != nil {
		return "", err
	}
	start, err := boundedTranscriptStart(file, 0, size)
	if err != nil {
		return "", err
	}
	if _, err := file.Seek(start, io.SeekStart); err != nil {
		return "", err
	}
	reader := bufio.NewReader(io.LimitReader(file, size-start))
	moved := ""
	for {
		// A line still being written has no newline yet and is left unread:
		// only a complete prompt or reply shows the old session went on, and
		// refusing on a write in flight would fail the agent for good.
		line, err := reader.ReadBytes('\n')
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		var record struct {
			Type        string `json:"type"`
			SessionID   string `json:"sessionId"`
			ContinuedIn string `json:"continuedInSessionId"`
		}
		if err := json.Unmarshal(line, &record); err != nil {
			return "", fmt.Errorf("decode transcript record: %w", err)
		}
		switch record.Type {
		case "continued-in":
			moved = ""
			if record.SessionID == session {
				moved = record.ContinuedIn
			}
		case "user", "assistant":
			moved = ""
		}
	}
	return moved, nil
}
