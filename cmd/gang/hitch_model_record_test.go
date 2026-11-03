//go:build acceptance

package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestHitchPersistsExplicitModelOptions(t *testing.T) {
	err, _, f := windowHitch(t, true, "--window", "7", "--model", "vendor/model", "--effort", "high")
	if err == nil || !strings.Contains(err.Error(), "spawn pane") {
		t.Fatalf("launch barrier: %v", err)
	}
	agents, err := f.run.team.ListAgents()
	if err != nil {
		t.Fatal(err)
	}
	if len(agents) != 1 {
		t.Fatalf("records: %+v", agents)
	}
	data, err := json.Marshal(agents[0])
	if err != nil {
		t.Fatal(err)
	}
	var saved map[string]any
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatal(err)
	}
	if saved["configured_model"] != "vendor/model" || saved["configured_effort"] != "high" {
		t.Fatalf("saved launch options: %s", data)
	}
}
