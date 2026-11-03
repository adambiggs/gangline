package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

func summaryPage(t *testing.T, data string, budget int64) logMetadataPage {
	t.Helper()
	r := &boundedMetadataReader{Reader: strings.NewReader(data)}
	p, err := readLogMetadata(r, r.Size(), logFilter{Summary: true, MaxBytes: budget}, time.Date(2026, 11, 3, 1, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if int64(r.read) > budget+1 {
		t.Fatalf("tail scanned beyond budget: %d > %d", r.read, budget+1)
	}
	return p
}

func TestLogSummaryBoundsTailAndRetainsExactEvidence(t *testing.T) {
	line := metadataUsage + "\n"
	data := strings.Repeat(line, 100)
	budget := int64(len(line) + 17)
	p := summaryPage(t, data, budget)
	s := summarizeLogMetadata(p)
	if len(s.Subjects) != 1 || len(s.Subjects[0].Usage) != 1 || !s.OmittedHead || s.More ||
		s.Offset != int64(len(data))-budget || s.Next != int64(len(data)) ||
		len(s.Gaps) != 1 || s.Gaps[0].Kind != "tail_starts_inside_record" {
		t.Fatalf("wrong bounded tail: %+v", s)
	}
	if s.SHA256 != fmt.Sprintf("%x", sha256.Sum256([]byte(data[s.Offset:s.ReadEnd]))) {
		t.Fatal("hash does not identify exactly the bytes read")
	}
	if s.Subjects[0].Usage[0].Offset != int64(len(data)-len(line)) {
		t.Fatal("summary changed original record offset")
	}
	// An exactly aligned tail must not drop its first complete record.
	s = summarizeLogMetadata(summaryPage(t, data, int64(len(line))))
	if len(s.Subjects) != 1 || len(s.Gaps) != 0 {
		t.Fatal("aligned tail lost a complete record")
	}
}

func TestLogSummaryNoCompleteRecordRemainsUnknown(t *testing.T) {
	for _, data := range []string{"", "unterminated", metadataUsage, metadataUsage + "\n"} {
		s := summarizeLogMetadata(summaryPage(t, data, 8))
		if len(s.Subjects) != 0 {
			t.Fatal("incomplete or oversized record became a measurement")
		}
		if data != "" && len(s.Gaps) == 0 {
			t.Fatal("unread record did not expose its gap")
		}
	}
}

func TestLogSummaryDoesNotSumRepeatOrHideUnknown(t *testing.T) {
	data := strings.Repeat(metadataUsage+"\n", 100)
	s := summarizeLogMetadata(summaryPage(t, data, int64(len(data))))
	a := s.Subjects[0]
	if len(a.Usage) != 1 || a.Usage[0].Reading.Usage.Output == nil || *a.Usage[0].Reading.Usage.Output != 0 ||
		a.Usage[0].Reading.Usage.Input != nil || a.WindowUsage != nil || a.StateDuration != nil {
		t.Fatal("repeat or unknown became a total/zero/duration")
	}
	unknown := `{"type":"observation","at":"2026-11-03T00:10:00Z","hitch_id":"h1","readings":[{"kind":"token-usage","source":"status-line","session_id":"s1","status":"unknown"}]}`
	data += unknown + "\n"
	s = summarizeLogMetadata(summaryPage(t, data, int64(len(data))))
	if len(s.Subjects[0].Usage) != 1 || s.Subjects[0].Usage[0].Reading.Status != "unknown" || s.Subjects[0].Usage[0].Reading.Usage != nil {
		t.Fatal("older observed reading concealed latest unknown")
	}
}

func TestLogSummarySeparatesRegistrationSessionAndSource(t *testing.T) {
	data := metadataUsage + "\n" + strings.ReplaceAll(metadataUsage, `"h1"`, `"h2"`) + "\n" +
		strings.ReplaceAll(metadataUsage, `"s1"`, `"s2"`) + "\n" + strings.ReplaceAll(metadataUsage, "status-line", "session-log") + "\n"
	s := summarizeLogMetadata(summaryPage(t, data, int64(len(data))))
	if len(s.Subjects) != 2 || len(s.Subjects[0].Usage) != 3 || len(s.Subjects[1].Usage) != 1 || s.Subjects[0].Name != s.Subjects[1].Name {
		t.Fatal("namesake/source/session observations were merged")
	}
	// Even repeated missing-session measurements cannot be joined as one source.
	data = strings.Repeat(strings.ReplaceAll(metadataUsage, `,"session_id":"s1"`, "")+"\n", 2)
	s = summarizeLogMetadata(summaryPage(t, data, int64(len(data))))
	if len(s.Subjects[0].Usage) != 2 {
		t.Fatal("missing identity was used as a dedup identity")
	}
}

func TestLogSummaryRetainsBoundariesGapsAndNoFreshnessInference(t *testing.T) {
	data := `{"type":"activity_observed","at":"2026-11-03T00:01:00Z","hitch_id":"h1","activity":"busy","reason":"PRIVATE"}` + "\n" +
		`{"type":"watchdog_unavailable","at":"2026-11-03T00:02:00Z","reason":"PRIVATE"}` + "\n" +
		`{"type":"activity_observed","at":"2026-11-03T00:00:00Z","hitch_id":"h1","activity":"idle"}` + "\n" +
		`{"type":"drop_finished","at":"2026-11-03T00:03:00Z","hitch_id":"h1"}` + "\nPRIVATE_TAIL"
	s := summarizeLogMetadata(summaryPage(t, data, int64(len(data))))
	if len(s.Boundaries) != 2 || len(s.Gaps) != 2 || !s.More || s.OmittedHead ||
		s.Subjects[0].Activity.Activity != "idle" || s.Subjects[0].Activity.At.Equal(s.ReadAt) || s.Subjects[0].StateDuration != nil {
		t.Fatalf("boundaries/order/unknown durations lost: %+v", s)
	}
	b, err := json.Marshal(s)
	if err != nil || bytes.Contains(b, []byte("PRIVATE")) || !bytes.Contains(b, []byte(`"window_usage":null`)) {
		t.Fatal("summary disclosed excluded text or hid unknown window usage")
	}
}

func TestLogSummaryOptions(t *testing.T) {
	for _, args := range [][]string{
		{"--summary"}, {"--summary", "--all", "log"}, {"--summary", "--limit", "1", "log"},
		{"--summary", "--metadata", "log"}, {"--summary", "--offset", "0", "log"},
		{"--summary", "--agent", "h1", "log"}, {"--summary", "--type", "observation", "log"},
		{"--summary", "--max-bytes", "1048577", "log"},
	} {
		if _, _, err := parseLogFilter(args, true); err == nil {
			t.Fatal("accepted unsafe summary options", args)
		}
	}
	f, files, err := parseLogFilter([]string{"--summary", "--max-bytes", "128", "log"}, true)
	if err != nil || !f.Summary || f.Metadata || f.MaxBytes != 128 || len(files) != 1 {
		t.Fatal("summary options refused", err)
	}
}

func BenchmarkLogSummary(b *testing.B) {
	line := metadataUsage + "\n"
	for _, size := range []int{64 * 1024, 1024 * 1024} {
		b.Run(fmt.Sprint(size), func(b *testing.B) {
			data := strings.Repeat(line, size/len(line)+1)
			b.SetBytes(int64(size))
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				p, err := readLogMetadata(strings.NewReader(data), int64(len(data)), logFilter{Summary: true, MaxBytes: int64(size)}, time.Time{})
				if err != nil {
					b.Fatal(err)
				}
				_ = summarizeLogMetadata(p)
			}
		})
	}
}
