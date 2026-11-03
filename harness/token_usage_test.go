package harness

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func findTokenUsage(t *testing.T, readings []Reading) Reading {
	t.Helper()
	for _, r := range readings {
		if r.Kind == "token-usage" {
			return r
		}
	}
	t.Fatal("token usage absent")
	return Reading{}
}

func TestStatuslineSessionUsageIsNotContextOccupancy(t *testing.T) {
	payload := `{"session_id":"s","context_window":{"context_window_size":200000,"total_input_tokens":12345,"total_output_tokens":321,"current_usage":{"input_tokens":50,"cache_creation_input_tokens":20,"cache_read_input_tokens":30}}}`
	_, readings, err := ReadStatusline([]byte(payload))
	if err != nil {
		t.Fatal(err)
	}
	u := findTokenUsage(t, readings)
	if u.SessionID != "s" || u.Status != "observed" || u.At != nil || u.SourceOffset != 0 ||
		u.Usage.Semantics != "session-cumulative" || u.Usage.CacheInclusion != "unverified" ||
		*u.Usage.Input != 12345 || *u.Usage.Output != 321 || u.Usage.CachedInput != nil {
		t.Fatalf("source usage lost or reinterpreted: %+v", u)
	}
	if readings[0].Kind != "context" || *readings[0].Used != 100 || *readings[0].Limit != 200000 {
		t.Fatalf("occupancy changed: %+v", readings[0])
	}
	// Repeated callbacks carry the same cumulative values, never a fabricated
	// delta, timestamp, cache total or task attribution.
	_, again, err := ReadStatusline([]byte(payload))
	if err != nil {
		t.Fatal(err)
	}
	a, _ := json.Marshal(u)
	b, _ := json.Marshal(findTokenUsage(t, again))
	if string(a) != string(b) {
		t.Fatal("repeated native counter changed")
	}
}

func TestStatuslineUsageMissingPartialAndZero(t *testing.T) {
	for _, tc := range []struct {
		fields, status string
		input, output  bool
	}{
		{``, "unknown", false, false},
		{`,"total_input_tokens":0`, "partial", true, false},
		{`,"total_output_tokens":0`, "partial", false, true},
		{`,"total_input_tokens":0,"total_output_tokens":0`, "observed", true, true},
	} {
		_, readings, err := ReadStatusline([]byte(`{"session_id":"s","context_window":{"current_usage":null` + tc.fields + `}}`))
		if err != nil {
			t.Fatal(err)
		}
		u := findTokenUsage(t, readings)
		if u.Status != tc.status || (u.Usage.Input != nil) != tc.input || (u.Usage.Output != nil) != tc.output {
			t.Fatalf("unknown became zero or partial lost: %+v", u)
		}
	}
	for _, value := range []string{`-1`, `1.5`, `"10"`} {
		_, readings, err := ReadStatusline([]byte(`{"session_id":"s","context_window":{"context_window_size":200000,"current_usage":{"input_tokens":5,"cache_creation_input_tokens":0,"cache_read_input_tokens":0},"total_input_tokens":` + value + `}}`))
		if err != nil {
			t.Fatal(err)
		}
		usage := findTokenUsage(t, readings)
		if usage.Status != "unknown" || usage.Usage.Input != nil || *readings[0].Used != 5 {
			t.Fatalf("bad counter hid valid context or became usage: %+v", readings)
		}
	}
}

func TestTranscriptCumulativeUsageIdentityAndCursor(t *testing.T) {
	const header = "{\"type\":\"session_meta\",\"payload\":{\"id\":\"s\"}}\n"
	const line = `{"timestamp":"2026-11-03T00:00:00Z","type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"total_tokens":40},"model_context_window":200000,"total_token_usage":{"input_tokens":1000,"output_tokens":50,"cached_input_tokens":700}}}}` + "\n"
	primitive := Invocation{Name: "codex-session-log"}
	parsed, err := ReadTranscript(primitive, strings.NewReader(header+line), "s", 0, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	u := findTokenUsage(t, parsed.Readings)
	if u.SessionID != "s" || u.SourceOffset != int64(len(header+line)) || u.At == nil ||
		u.At.Format(time.RFC3339) != "2026-11-03T00:00:00Z" || *u.Usage.Input != 1000 ||
		*u.Usage.Output != 50 || *u.Usage.CachedInput != 700 || u.Usage.CacheInclusion != "unverified" {
		t.Fatalf("source counter/identity lost: %+v", u)
	}
	if *parsed.Readings[0].Used != 40 {
		t.Fatal("cumulative usage substituted for context occupancy")
	}
	again, err := ReadTranscript(primitive, strings.NewReader(header+line), "s", parsed.Offset, time.Time{})
	if err != nil || len(again.Readings) != 0 {
		t.Fatalf("cursor replayed usage: %+v %v", again, err)
	}
	// Lower source counters must remain visible to a later segment-aware
	// aggregator. This adapter must neither clamp nor silently add them.
	lower := strings.ReplaceAll(line, `"input_tokens":1000`, `"input_tokens":20`)
	parsed, err = ReadTranscript(primitive, strings.NewReader(header+line+lower), "s", parsed.Offset, time.Time{})
	if err != nil || *findTokenUsage(t, parsed.Readings).Usage.Input != 20 {
		t.Fatalf("reset lost: %+v %v", parsed, err)
	}
	negative := strings.ReplaceAll(line, `"output_tokens":50`, `"output_tokens":-1`)
	bad, err := ReadTranscript(primitive, strings.NewReader(header+negative), "s", 0, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	invalid := findTokenUsage(t, bad.Readings)
	if invalid.Status != "unknown" || invalid.Usage.Output != nil || bad.Offset != int64(len(header+negative)) || *bad.Readings[0].Used != 40 {
		t.Fatalf("invalid usage stalled cursor or hid context: %+v", bad)
	}
}
