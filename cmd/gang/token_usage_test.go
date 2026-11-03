package main

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
)

func TestUsageReadingSurvivesAuditNormalization(t *testing.T) {
	_, readings, err := harness.ReadStatusline([]byte(`{"session_id":"native-s","context_window":{"total_input_tokens":90,"total_output_tokens":0}}`))
	if err != nil {
		t.Fatal(err)
	}
	event := core.Event{Type: "observation", At: time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC), HitchID: "hitch-a"}
	for _, r := range readings {
		event.Readings = append(event.Readings, coreReading(r))
	}
	data, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	if err := core.ValidateEventJSON(data); err != nil {
		t.Fatal(err)
	}
	var restored core.Event
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}
	usage := restored.Readings[2]
	if restored.HitchID != "hitch-a" || usage.SessionID != "native-s" || usage.Usage == nil ||
		*usage.Usage.Input != 90 || *usage.Usage.Output != 0 || usage.Usage.CachedInput != nil || usage.At != nil {
		t.Fatalf("audit lost source semantics: %+v", usage)
	}
}
