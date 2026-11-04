//go:build acceptance

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
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
	p, err := f.run.team.Agent(agents[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(p.Directory, "model-metadata.json"))
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
	if agents[0].ConfiguredModel != "" || agents[0].ConfiguredEffort != "" {
		t.Fatal("failed launch without a registration attributed configuration")
	}
}
