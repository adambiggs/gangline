package harness

import (
	"strings"
	"testing"
	"time"
)

func TestTranscriptRetainsSkippedWindowBoundary(t *testing.T) {
	header := "{\"type\":\"session_meta\",\"payload\":{\"id\":\"s\"}}\n"
	ignored := "{\"type\":\"response_item\"}\n"
	tail := "{\"timestamp\":\"2026-11-03T00:00:00Z\",\"type\":\"event_msg\",\"payload\":{\"type\":\"context_compacted\"}}\n"
	data := header + strings.Repeat(ignored, transcriptWindow/len(ignored)+2) + tail
	for _, offset := range []int64{0, int64(len(header))} {
		got, err := ReadTranscript(Invocation{Name: "codex-session-log"}, strings.NewReader(data), "s", offset, time.Time{})
		if err != nil {
			t.Fatal(err)
		}
		if len(got.Readings) != 2 || got.Readings[0].Kind != "observation-gap" ||
			got.Readings[0].NativeEvent != "source-window-skipped" || got.Readings[0].SessionID != "s" ||
			got.Readings[0].Status != "unknown" || got.Readings[0].At != nil || got.Readings[0].SourceOffset <= int64(len(header)) {
			t.Fatalf("unread source bytes were skipped without an identity-bound gap: %+v", got.Readings)
		}
		if got.Readings[1].Kind != "compaction-finished" || got.Readings[1].SessionID != "s" ||
			got.Readings[1].SourceOffset != int64(len(data)) || got.Offset != int64(len(data)) {
			t.Fatalf("compaction lost its source boundary: %+v", got)
		}
		next, err := ReadTranscript(Invocation{Name: "codex-session-log"}, strings.NewReader(data), "s", got.Offset, time.Time{})
		if err != nil || len(next.Readings) != 0 || next.Offset != got.Offset {
			t.Fatal("unchanged source replayed a gap or compaction", err)
		}
	}
}
