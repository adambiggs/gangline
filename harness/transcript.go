package harness

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
)

const transcriptWindow = 4 << 20

func transcriptHeader(input io.ReadSeeker, session string) (int64, error) {
	if _, err := input.Seek(0, io.SeekStart); err != nil {
		return 0, err
	}
	first, err := bufio.NewReader(io.LimitReader(input, transcriptWindow)).ReadBytes('\n')
	if err != nil {
		return 0, fmt.Errorf("read session metadata: %w", err)
	}
	var meta struct {
		Type    string `json:"type"`
		Payload struct {
			ID string `json:"id"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(first, &meta); err != nil {
		return 0, err
	}
	if meta.Type != "session_meta" || meta.Payload.ID != session {
		return 0, fmt.Errorf("session log metadata does not match native session %q", session)
	}
	return int64(len(first)), nil
}

// Native observations use a bounded tail. Missing evidence stays unknown;
// the amount of settled conversation never determines a command's work.
func boundedTranscriptStart(input io.ReadSeeker, start, size int64) (int64, error) {
	if size-start <= transcriptWindow {
		return start, nil
	}
	start = size - transcriptWindow
	if _, err := input.Seek(start, io.SeekStart); err != nil {
		return 0, err
	}
	partial, err := bufio.NewReader(io.LimitReader(input, size-start)).ReadBytes('\n')
	if err != nil && err != io.EOF {
		return 0, err
	}
	return start + int64(len(partial)), nil
}
