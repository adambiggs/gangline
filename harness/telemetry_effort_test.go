package harness

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestTurnContextRetainsReportedEffortWithoutGuessing(t *testing.T) {
	for _, effort := range []string{"high", ""} {
		payload := map[string]string{"model": "vendor/full-model", "effort": effort}
		if effort == "" {
			delete(payload, "effort")
		}
		line, _ := json.Marshal(map[string]any{"timestamp": "2026-11-03T12:00:00Z", "type": "turn_context", "payload": payload})
		input := "{\"type\":\"session_meta\",\"payload\":{\"id\":\"session\"}}\n" + string(line) + "\n"
		parsed, err := ReadTranscript(Invocation{Name: "codex-session-log"}, strings.NewReader(input), "session", 0, time.Time{})
		if err != nil {
			t.Fatal(err)
		}
		if len(parsed.Readings) != 1 || parsed.Readings[0].Model != "vendor/full-model" {
			t.Fatalf("readings: %+v", parsed.Readings)
		}
		data, _ := json.Marshal(parsed.Readings[0])
		var got map[string]any
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatal(err)
		}
		if effort != "" && got["effort"] != effort {
			t.Fatalf("effort lost: %s", data)
		}
		if effort == "" && got["effort"] != nil {
			t.Fatalf("effort invented: %s", data)
		}
	}
}

func TestTurnContextRejectsMalformedEffort(t *testing.T) {
	input := "{\"type\":\"session_meta\",\"payload\":{\"id\":\"session\"}}\n" +
		"{\"timestamp\":\"2026-11-03T12:00:00Z\",\"type\":\"turn_context\",\"payload\":{\"model\":\"vendor/model\",\"effort\":7}}\n"
	if _, err := ReadTranscript(Invocation{Name: "codex-session-log"}, strings.NewReader(input), "session", 0, time.Time{}); err == nil {
		t.Fatal("non-string effort accepted")
	}
}
