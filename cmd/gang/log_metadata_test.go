package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

const metadataUsage = `{"type":"observation","at":"2026-11-03T00:00:00Z","hitch_id":"h1","name":"worker","reason":"PRIVATE_REASON","readings":[{"kind":"token-usage","source":"status-line","status":"partial","reason":"PRIVATE_READING","session_id":"s1","usage":{"semantics":"session-cumulative","fields":"context_window.total_input_tokens,total_output_tokens","cache_inclusion":"unverified","output_tokens":0}}]}`

func metadataPage(t *testing.T, data string, offset, budget int64) logMetadataPage {
	t.Helper()
	p, err := readLogMetadata(strings.NewReader(data), int64(len(data)), logFilter{Offset: offset, MaxBytes: budget}, time.Date(2026, 11, 3, 1, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLogMetadataExcludesBodiesAndPreservesUnknownZero(t *testing.T) {
	message := `{"type":"send_queued","at":"2026-11-03T00:00:00Z","envelope":{"id":"e","from":{"kind":"gangline","name":"system"},"to":"worker","recipient":"h1","message":{"text":"PRIVATE_MESSAGE"},"created_at":"2026-11-03T00:00:00Z"}}`
	compaction := `{"type":"compaction_requested","at":"2026-11-03T00:00:00Z","compaction":{"id":"c","resume":{"text":"PRIVATE_CONTINUATION"},"started_at":"2026-11-03T00:00:00Z","deadline":"2026-11-03T00:01:00Z","status":"queued"}}`
	data := metadataUsage + "\n" + message + "\n" + compaction + "\n"
	p := metadataPage(t, data, 0, int64(len(data)))
	b, err := json.Marshal(p)
	if err != nil || bytes.Contains(b, []byte("PRIVATE")) || bytes.Contains(b, []byte(`"reason"`)) {
		t.Fatalf("metadata disclosed excluded text: error %v", err)
	}
	if len(p.Events) != 2 || len(p.Gaps) != 0 || p.More || p.Next != int64(len(data)) {
		t.Fatalf("wrong page shape: %+v", p)
	}
	u := p.Events[0].Usage[0]
	if u.Status != "partial" || u.SessionID != "s1" || u.Usage.Input != nil || u.Usage.Output == nil || *u.Usage.Output != 0 {
		t.Fatal("missing counter became zero or observed zero was lost")
	}
}

func TestLogMetadataPagesRetainByteBoundaries(t *testing.T) {
	line := metadataUsage + "\n"
	data := line + line
	tooSmall := metadataPage(t, data, 0, 8)
	if tooSmall.Next != 0 || len(tooSmall.Gaps) != 1 || tooSmall.Gaps[0].Kind != "record_exceeds_budget" {
		t.Fatal("oversized first record did not explain lack of progress")
	}
	first := metadataPage(t, data, 0, int64(len(line)+7))
	if first.Next != int64(len(line)) || first.ReadEnd != int64(len(line)+7) || !first.More || len(first.Events) != 1 ||
		len(first.Gaps) != 1 || first.Gaps[0].Kind != "page_ends_inside_record" {
		t.Fatal("partial record was consumed or hidden")
	}
	second := metadataPage(t, data, first.Next, int64(len(line)))
	if second.More || second.Events[0].Offset != first.Next || second.Next != int64(len(data)) {
		t.Fatal("page continuation skipped or repeated a complete record")
	}
	tail := metadataPage(t, metadataUsage, 0, int64(len(metadataUsage)))
	if tail.Next != 0 || len(tail.Events) != 0 || len(tail.Gaps) != 1 || tail.Gaps[0].Kind != "incomplete_tail" {
		t.Fatal("unterminated record treated as settled evidence")
	}
	for _, offset := range []int64{1, int64(len(data) + 1)} {
		if _, err := readLogMetadata(strings.NewReader(data), int64(len(data)), logFilter{Offset: offset, MaxBytes: 100}, time.Time{}); err == nil {
			t.Fatal("invalid continuation was accepted")
		}
	}
}

func BenchmarkLogMetadataPage(b *testing.B) {
	line := metadataUsage + "\n"
	for _, size := range []int{64 * 1024, 1024 * 1024} {
		b.Run(fmt.Sprint(size), func(b *testing.B) {
			data := strings.Repeat(line, size/len(line)+1)
			b.SetBytes(int64(size))
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				if _, err := readLogMetadata(strings.NewReader(data), int64(len(data)), logFilter{MaxBytes: int64(size)}, time.Time{}); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func TestLogMetadataGapsDoNotExposeDecoderText(t *testing.T) {
	data := "PRIVATE_MALFORMED\n" + metadataUsage + "\n" + strings.Replace(metadataUsage, "2026-11-03", "2026-11-02", 1) + "\n"
	p := metadataPage(t, data, 0, int64(len(data)))
	if len(p.Events) != 2 || len(p.Gaps) != 2 || p.Gaps[0].Kind != "invalid_audit_record" || p.Gaps[1].Kind != "observation_time_out_of_order" {
		t.Fatal("damage or clock regression was hidden")
	}
	b, _ := json.Marshal(p)
	if bytes.Contains(b, []byte("PRIVATE")) {
		t.Fatal("source text escaped through diagnostics")
	}
}

type boundedMetadataReader struct {
	*strings.Reader
	read int
}

func (r *boundedMetadataReader) ReadAt(p []byte, offset int64) (int, error) {
	r.read += len(p)
	return r.Reader.ReadAt(p, offset)
}

func TestLogMetadataReadBudgetDoesNotScanHistory(t *testing.T) {
	line := metadataUsage + "\n"
	r := &boundedMetadataReader{Reader: strings.NewReader(strings.Repeat(line, 100))}
	budget := int64(len(line))
	p, err := readLogMetadata(r, r.Size(), logFilter{Offset: budget, MaxBytes: budget}, time.Time{})
	if err != nil || r.read != len(line)+1 || len(p.Events) != 1 {
		t.Fatalf("read exceeded one page and boundary byte: bytes=%d err=%v", r.read, err)
	}
}

func TestLogMetadataNormalizesFreeTextUsageMetadata(t *testing.T) {
	data := strings.ReplaceAll(metadataUsage, "status-line", "PRIVATE_SOURCE")
	data = strings.ReplaceAll(data, "context_window.total_input_tokens,total_output_tokens", "PRIVATE_FIELDS")
	data = strings.ReplaceAll(data, `"partial"`, `"PRIVATE_STATUS"`)
	p := metadataPage(t, data+"\n", 0, int64(len(data)+1))
	b, _ := json.Marshal(p)
	if bytes.Contains(b, []byte("PRIVATE")) || len(p.Events) != 1 || p.Events[0].Usage[0].Status != "unknown" {
		t.Fatal("free-text source or status was relayed")
	}
}

func TestLogMetadataOptions(t *testing.T) {
	for _, args := range [][]string{
		{"--metadata"}, {"--offset", "0", "log.jsonl"}, {"--metadata", "--all", "log.jsonl"},
		{"--metadata", "--limit", "10", "log.jsonl"}, {"--metadata", "--max-bytes", "1048577", "log.jsonl"},
		{"--metadata", "--offset", "-1", "log.jsonl"},
	} {
		if _, _, err := parseLogFilter(args, true); err == nil {
			t.Fatal("accepted invalid options", args)
		}
	}
	f, files, err := parseLogFilter([]string{"--metadata", "--offset", "10", "--max-bytes", "100", "log.jsonl"}, true)
	if err != nil || !f.Metadata || f.Offset != 10 || f.MaxBytes != 100 || fmt.Sprint(files) != "[log.jsonl]" {
		t.Fatal("valid metadata options failed", err)
	}
	if _, _, err := parseLogFilter([]string{"--metadata"}, false); err == nil {
		t.Fatal("metadata silently accepted on a route that cannot expose it")
	}
}
