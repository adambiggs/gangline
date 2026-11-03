package harness

import (
	"bytes"
	"io"
	"time"
)

const transcriptContextWindow = 64 << 10

// CompareTranscriptContext checks only the previously saved context record.
// A match is not continuity: other bytes and source identity are not compared.
// Legacy/unbound witnesses and unreadable/oversized records stay unknown.
func CompareTranscriptContext(input io.ReadSeeker, session string, cursor int64, witness Reading) string {
	if witness.Kind != "context" || witness.Source != "session-log" || witness.NativeEvent != "token_count" || witness.Status != "observed" ||
		session == "" || witness.SessionID != session || witness.SourceOffset <= 0 || witness.SourceOffset > cursor ||
		witness.At == nil || witness.Used == nil || witness.Limit == nil {
		return "unknown"
	}
	size, err := input.Seek(0, io.SeekEnd)
	if err != nil || cursor > size {
		return "unknown"
	}
	end := witness.SourceOffset
	start := max(int64(0), end-transcriptContextWindow)
	if _, err := input.Seek(start, io.SeekStart); err != nil {
		return "unknown"
	}
	data := make([]byte, end-start)
	if _, err := io.ReadFull(input, data); err != nil || data[len(data)-1] != '\n' {
		return "unknown"
	}
	boundary := bytes.LastIndexByte(data[:len(data)-1], '\n')
	if boundary < 0 && start > 0 {
		return "unknown"
	}
	// Reuse the native decoder on this one complete record, without replaying
	// its readings or exposing decoder/body text to an error or audit record.
	parsed, err := readTranscriptRecords(bytes.NewReader(data[boundary+1:]), session, start+int64(boundary+1), time.Time{})
	if err != nil {
		return "unknown"
	}
	for _, found := range parsed.Readings {
		if found.Kind != "context" || found.NativeEvent != "token_count" || found.At == nil || found.Used == nil || found.Limit == nil {
			continue
		}
		if !found.At.Equal(*witness.At) || *found.Used != *witness.Used || *found.Limit != *witness.Limit {
			return "mismatch"
		}
		return "match"
	}
	return "unknown"
}
