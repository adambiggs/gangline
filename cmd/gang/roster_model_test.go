package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
)

func TestRosterModelFieldsKeepConfiguredAndObservedSeparate(t *testing.T) {
	for _, tc := range []struct {
		name, state string
		want        map[string]any
	}{
		{"legacy", `{"collar":"codex-qwen"}`, map[string]any{
			"configured_model": nil, "configured_effort": nil, "observed_model": nil, "observed_effort": nil}},
		{"qualified", `{"configured_model":"vendor/model-v1","configured_effort":"high","native":{"model":"vendor/model-v2","effort":"low"}}`, map[string]any{
			"configured_model": "vendor/model-v1", "configured_effort": "high", "observed_model": "vendor/model-v2", "observed_effort": "low"}},
		{"alias", `{"configured_model":"haiku","native":{"model":"old-model","effort":"old-effort","context":{"source":"status-line","model":"claude-native-id"}}}`, map[string]any{
			"configured_model": "haiku", "configured_effort": nil, "observed_model": "claude-native-id", "observed_effort": nil}},
		{"screen-is-not-native", `{"native":{"context":{"source":"screen","model":"guessed"}}}`, map[string]any{
			"configured_model": nil, "configured_effort": nil, "observed_model": nil, "observed_effort": nil}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var agent core.Agent
			if err := json.Unmarshal([]byte(tc.state), &agent); err != nil {
				t.Fatal(err)
			}
			agent.Name, agent.ID, agent.Activity = "worker", "fixture", core.Busy
			row, err := (&runtime{}).agentRow(agent)
			if err != nil {
				t.Fatal(err)
			}
			data, err := json.Marshal(row)
			if err != nil {
				t.Fatal(err)
			}
			var got map[string]any
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatal(err)
			}
			for field, want := range tc.want {
				value, present := got[field]
				if !present || value != want {
					t.Errorf("%s = %v (present %v), want %v", field, value, present, want)
				}
			}
			if got["name"] != "worker" || got["hitch_id"] != "fixture" || got["activity"] != "busy" {
				t.Fatalf("existing fields changed: %s", data)
			}
		})
	}
}

func TestNativeEffortIsClearedWhenNextModelReadingOmitsIt(t *testing.T) {
	input := "{\"type\":\"session_meta\",\"payload\":{\"id\":\"session\"}}\n" +
		"{\"timestamp\":\"2026-11-03T12:00:00Z\",\"type\":\"turn_context\",\"payload\":{\"model\":\"vendor/model\",\"effort\":\"high\"}}\n"
	parsed, err := harness.ReadTranscript(harness.Invocation{Name: "codex-session-log"}, strings.NewReader(input), "session", 0, time.Time{})
	if err != nil || len(parsed.Readings) != 1 {
		t.Fatal(err)
	}
	first := coreReading(parsed.Readings[0])
	event, err := core.EncodeEvent(core.Event{Type: "observation", At: time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC), HitchID: "fixture", Readings: []core.Reading{first}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(event), `"effort"`) {
		t.Fatalf("existing audit schema changed: %s", event)
	}
	var state core.NativeState
	acceptReadings(&state, []core.Reading{first})
	data, _ := json.Marshal(state)
	var fields map[string]any
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatal(err)
	}
	if fields["effort"] != "high" {
		t.Fatalf("native effort lost: %s", data)
	}
	acceptReadings(&state, []core.Reading{{Kind: "model", Source: "session-log", Model: "vendor/next"}})
	data, _ = json.Marshal(state)
	fields = nil
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatal(err)
	}
	if fields["effort"] != nil {
		t.Fatalf("old effort carried into unknown turn: %s", data)
	}
}
