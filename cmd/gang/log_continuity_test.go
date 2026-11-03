package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func continuityUsage(source string, cursor, input, output int) string {
	return fmt.Sprintf(`{"type":"observation","at":"2026-11-03T00:00:00Z","hitch_id":"h1","readings":[{"kind":"token-usage","source":%q,"session_id":"s1","source_offset":%d,"status":"observed","usage":{"semantics":"session-cumulative","fields":"info.total_token_usage","cache_inclusion":"unverified","input_tokens":%d,"output_tokens":%d}}]}`+"\n", source, cursor, input, output)
}

func TestLogContinuityMakesCounterAndCursorBreaksExplicit(t *testing.T) {
	first := continuityUsage("session-log", 100, 90, 5)
	for _, tc := range []struct{ name, next, gap string }{
		{"counter decrease", continuityUsage("session-log", 200, 80, 6), "cumulative_counter_decreased"},
		{"output decrease", continuityUsage("session-log", 200, 100, 4), "cumulative_counter_decreased"},
		{"cursor rewind", continuityUsage("session-log", 90, 100, 6), "source_offset_rewind"},
		{"conflicting replay", continuityUsage("session-log", 100, 100, 6), "source_offset_conflict"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := first + tc.next
			p := metadataPage(t, data, 0, int64(len(data)))
			if len(p.Gaps) != 1 || p.Gaps[0].Kind != tc.gap || p.Gaps[0].Offset != int64(len(first)) {
				t.Fatalf("ambiguous source continuity concealed: %+v", p.Gaps)
			}
			s := summarizeLogMetadata(p)
			if len(s.Gaps) != 1 || s.Subjects[0].WindowUsage != nil || s.Subjects[0].StateDuration != nil {
				t.Fatal("summary lost the gap or invented totals/duration")
			}
		})
	}
}

func TestLogContinuityRetainsNativeBoundariesWithoutText(t *testing.T) {
	for _, kind := range []string{"compaction-checkpoint", "compaction-finished", "observation-gap"} {
		t.Run(kind, func(t *testing.T) {
			nativeEvent, status := "source-window-skipped", "unknown"
			if kind == "compaction-checkpoint" {
				nativeEvent, status = "compaction_checkpoint", "observed"
			} else if kind == "compaction-finished" {
				nativeEvent, status = "context_compacted", "observed"
			}
			data := fmt.Sprintf(`{"type":"observation","at":"2026-11-03T00:00:00Z","hitch_id":"h1","reason":"PRIVATE","readings":[{"kind":%q,"source":"session-log","session_id":"s1","source_offset":42,"status":%q,"native_event":%q,"reason":"PRIVATE"}]}`+"\n", kind, status, nativeEvent)
			s := summarizeLogMetadata(summaryPage(t, data, int64(len(data))))
			if len(s.Boundaries) != 1 || s.Boundaries[0].HitchID != "h1" || s.Boundaries[0].Offset != 0 {
				t.Fatalf("native boundary disappeared from saved history: %+v", s)
			}
			b, err := json.Marshal(s)
			if err != nil || strings.Contains(string(b), "PRIVATE") || !strings.Contains(string(b), `"source_offset":42`) {
				t.Fatal("boundary lost source identity or exposed text", err)
			}
		})
	}
}

func TestLogContinuityReplayAndIdentityDoNotInventReset(t *testing.T) {
	first := continuityUsage("session-log", 100, 90, 5)
	next := continuityUsage("session-log", 200, 100, 6)
	for _, data := range []string{
		strings.Repeat(first, 100),
		first + next + first + next, // Source state save replay, including an older offset.
		first + strings.ReplaceAll(continuityUsage("session-log", 90, 1, 1), `"s1"`, `"s2"`),
		first + strings.ReplaceAll(continuityUsage("session-log", 90, 1, 1), `"h1"`, `"h2"`),
		first + continuityUsage("status-line", 90, 1, 1),
	} {
		p := metadataPage(t, data, 0, int64(len(data)))
		if len(p.Gaps) != 0 {
			t.Fatalf("replay or a separate identity became a reset: %+v", p.Gaps)
		}
		for _, a := range summarizeLogMetadata(p).Subjects {
			if a.WindowUsage != nil || a.StateDuration != nil {
				t.Fatal("snapshots became usage or elapsed time")
			}
		}
	}
}

func TestLogContinuityUnknownAndPageStartAreNotZeroBaseline(t *testing.T) {
	first := continuityUsage("status-line", 100, 90, 5)
	unknown := `{"type":"observation","at":"2026-11-03T00:00:00Z","hitch_id":"h1","readings":[{"kind":"token-usage","source":"status-line","session_id":"s1","status":"unknown"}]}` + "\n"
	last := continuityUsage("status-line", 200, 1, 1)
	for _, data := range []string{first + unknown + last, last} {
		p := metadataPage(t, data, 0, int64(len(data)))
		if len(p.Gaps) != 0 || summarizeLogMetadata(p).Subjects[0].WindowUsage != nil {
			t.Fatal("unknown or absent baseline used for a delta")
		}
	}
	data := first + last
	p := metadataPage(t, data, int64(len(first)), int64(len(last)))
	if len(p.Gaps) != 0 {
		t.Fatal("page acquired comparison state from outside its read window")
	}
}

func TestLogContinuityLifecycleOutageBoundariesSurvive(t *testing.T) {
	for _, kind := range []string{"rehitch_started", "session_bound", "clear_completed", "compaction_completed", "watchdog_unavailable", "watchdog_available", "tick_failed"} {
		t.Run(kind, func(t *testing.T) {
			line := fmt.Sprintf(`{"type":%q,"at":"2026-11-03T00:00:00Z","hitch_id":"h1","name":"worker","source":"watchdog","reason":"PRIVATE","binding":{"session_id":"s2","previous_session_id":"s1","collar":"codex"}}`+"\n", kind)
			data := continuityUsage("session-log", 100, 90, 5) + line
			s := summarizeLogMetadata(summaryPage(t, data, int64(len(data))))
			if len(s.Boundaries) != 1 || s.Boundaries[0].Type != kind || s.Subjects[0].StateDuration != nil {
				t.Fatal("existing lifecycle/outage boundary lost or extended to read time")
			}
		})
	}
}
