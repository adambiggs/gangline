package harness

import (
	"strings"
	"testing"
	"time"
)

const contextWitnessHeader = "{\"type\":\"session_meta\",\"payload\":{\"id\":\"s\"}}\n"
const contextWitnessLine = "{\"timestamp\":\"2026-09-22T10:00:00Z\",\"type\":\"event_msg\",\"payload\":{\"type\":\"token_count\",\"info\":{\"last_token_usage\":{\"total_tokens\":40},\"model_context_window\":200000,\"total_token_usage\":{\"input_tokens\":90}}}}\n"

func TestTranscriptContextWitnessComparison(t *testing.T) {
	data := contextWitnessHeader + contextWitnessLine
	parsed, err := ReadTranscript(Invocation{Name: "codex-session-log"}, strings.NewReader(data), "s", 0, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	witness := parsed.Readings[0]
	if witness.SessionID != "s" || witness.SourceOffset != int64(len(data)) {
		t.Fatal("context lost its cursor/session binding")
	}
	for _, tc := range []struct {
		name, data, want string
		change           func(*Reading)
	}{
		{"unchanged", data, "match", nil},
		{"appended", data + contextWitnessLine, "match", nil},
		{"used changed", strings.Replace(data, `"total_tokens":40`, `"total_tokens":60`, 1), "mismatch", nil},
		{"limit changed", strings.Replace(data, `"model_context_window":200000`, `"model_context_window":100000`, 1), "mismatch", nil},
		{"timestamp changed", strings.Replace(data, "10:00:00Z", "10:00:01Z", 1), "mismatch", nil},
		{"usage-only change undetected", strings.Replace(data, `"input_tokens":90`, `"input_tokens":20`, 1), "match", nil},
		{"model not a source witness", data, "match", func(r *Reading) { r.Model = "from-agent-state" }},
		{"legacy witness", data, "unknown", func(r *Reading) { r.SourceOffset = 0; r.SessionID = "" }},
		{"wrong session", data, "unknown", func(r *Reading) { r.SessionID = "other" }},
		{"compaction reset", data, "unknown", func(r *Reading) { r.Status = "unknown" }},
		{"another source", data, "unknown", func(r *Reading) { r.Source = "screen" }},
		{"missing timestamp", data, "unknown", func(r *Reading) { r.At = nil }},
		{"missing used", data, "unknown", func(r *Reading) { r.Used = nil }},
		{"not a complete line", strings.TrimSuffix(data, "\n") + " ", "unknown", nil},
		{"malformed", strings.Replace(data, `"total_tokens":40`, `"total_tokens":xx`, 1), "unknown", nil},
		{"not context", strings.Replace(data, "token_count", "other_event", 1), "unknown", nil},
		{"truncated", data[:len(data)-1], "unknown", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := witness
			if tc.change != nil {
				tc.change(&w)
			}
			input := &countingTranscript{Reader: strings.NewReader(tc.data)}
			got := CompareTranscriptContext(input, "s", parsed.Offset, w)
			if got != tc.want {
				t.Fatalf("%s want %s", got, tc.want)
			}
			if input.read > transcriptContextWindow {
				t.Fatalf("lookback exceeded byte budget: %d", input.read)
			}
		})
	}
	if got := CompareTranscriptContext(strings.NewReader(data), "s", witness.SourceOffset-1, witness); got != "unknown" {
		t.Fatal("context ahead of saved cursor became witness", got)
	}
}

func TestTranscriptContextLookbackIsBoundedAndOffsetSpecific(t *testing.T) {
	prefix := contextWitnessHeader + strings.Repeat("{\"type\":\"response_item\"}\n", 10000)
	data := prefix + contextWitnessLine
	parsed, err := ReadTranscript(Invocation{Name: "codex-session-log"}, strings.NewReader(data), "s", 0, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	witness := parsed.Readings[0]
	// Later out-of-order context must not displace the exact saved witness.
	later := strings.Replace(contextWitnessLine, "10:00:00Z", "09:59:59Z", 1)
	input := &countingTranscript{Reader: strings.NewReader(data + later)}
	if got := CompareTranscriptContext(input, "s", int64(len(data+later)), witness); got != "match" {
		t.Fatal(got)
	}
	if input.read != transcriptContextWindow {
		t.Fatalf("expected one bounded lookback, got %d", input.read)
	}
	// A valid but oversized token_count cannot be compared within this budget.
	huge := strings.TrimSuffix(contextWitnessLine, "\n") + strings.Repeat(" ", transcriptContextWindow) + "\n"
	input = &countingTranscript{Reader: strings.NewReader(contextWitnessHeader + huge)}
	witness.SourceOffset = int64(len(contextWitnessHeader + huge))
	if got := CompareTranscriptContext(input, "s", witness.SourceOffset, witness); got != "unknown" {
		t.Fatal(got)
	}
	if input.read != transcriptContextWindow {
		t.Fatal(input.read)
	}
}
