package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/adambiggs/gangline/core"
)

func TestRosterJSONCarriesAvailabilityAsFields(t *testing.T) {
	f := newStateFixture(t)
	worker := f.add(t, "a", "worker", "claude")
	f.add(t, "b", "other", "codex")
	p, err := f.run.team.Agent(worker.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, marker := range []string{filepath.Join(p.Directory, "process-unavailable"), filepath.Join(f.run.team.Directory, "watchdog-unavailable")} {
		if err := os.WriteFile(marker, nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.cmd.execute([]string{"roster", "--json"}); err != nil {
		t.Fatal(err)
	}
	var got rosterJSON
	decodeOutput(t, f, &got)
	if got.WatchdogAvailable || len(got.Agents) != 2 {
		t.Fatalf("roster = %+v", got)
	}
	byName := map[core.AgentName]agentJSON{}
	for _, a := range got.Agents {
		byName[a.Name] = a
	}
	if a := byName["worker"]; a.HitchID != "a" || a.Collar != "claude" || a.Status != core.Active || a.Pane != "worker" || a.ProcessAvailable {
		t.Fatalf("worker = %+v", a)
	}
	if a := byName["other"]; a.Collar != "codex" || !a.ProcessAvailable {
		t.Fatalf("other = %+v", a)
	}
	if err := f.cmd.execute([]string{"roster", "--porcelain"}); err == nil {
		t.Fatal("roster accepted --porcelain")
	}
}

func TestStatusJSONCarriesEvidenceAndCompaction(t *testing.T) {
	f := newStateFixture(t)
	worker := f.add(t, "a", "worker", "codex")
	saveAgent(t, f, worker.ID, func(a *core.Agent) {
		a.Compaction = &core.Compaction{ID: "c1", Status: "refused", Reason: "native refusal"}
	})
	if err := f.cmd.execute([]string{"status", "worker", "--json"}); err != nil {
		t.Fatal(err)
	}
	var got agentJSON
	decodeOutput(t, f, &got)
	if got.Name != "worker" || got.HitchID != "a" || got.Status != core.Active || !got.ProcessAvailable {
		t.Fatalf("status = %+v", got)
	}
	if got.Compaction == nil || *got.Compaction != (compactionJSON{ID: "c1", Status: "refused", Reason: "native refusal"}) {
		t.Fatalf("compaction = %+v", got.Compaction)
	}
}

func TestContextJSONCarriesExactTokens(t *testing.T) {
	f := newStateFixture(t)
	worker := f.add(t, "a", "worker", "codex")
	used, limit, percent := int64(123456), int64(272000), 45.38823529411765
	saveAgent(t, f, worker.ID, func(a *core.Agent) {
		a.Native.Context = core.Reading{Kind: "context", Source: "fixture", Model: "gpt-test", Status: "observed", Used: &used, Limit: &limit, Percent: &percent}
	})
	if err := f.cmd.execute([]string{"context", "worker", "--json"}); err != nil {
		t.Fatal(err)
	}
	var got contextJSON
	decodeOutput(t, f, &got)
	if got.Name != "worker" || got.Status != "observed" || got.Model != "gpt-test" || got.Used == nil || *got.Used != used || got.Limit == nil || *got.Limit != limit || got.Percent == nil || *got.Percent != percent || got.Source != "telemetry" {
		t.Fatalf("context = %+v", got)
	}
	for _, args := range [][]string{{"context", "--json", "--widget", "worker"}, {"context", "--json", "--clear"}} {
		if err := f.cmd.execute(args); err == nil {
			t.Fatalf("%v accepted", args)
		}
	}
}

func TestContextJSONReportsAnUnknownReading(t *testing.T) {
	f := newStateFixture(t)
	worker := f.add(t, "a", "worker", "codex")
	saveAgent(t, f, worker.ID, func(a *core.Agent) {
		a.Native.Context = core.Reading{Kind: "context", Source: "fixture", Status: "unknown", Reason: "no token count yet"}
	})
	err := f.cmd.execute([]string{"context", "worker", "--json"})
	var unknown commandError
	if !errors.As(err, &unknown) || unknown.status != exitUnknown {
		t.Fatalf("unknown reading exit = %v", err)
	}
	var got contextJSON
	decodeOutput(t, f, &got)
	if got.Name != "worker" || got.Status != "unknown" || got.Reason != "no token count yet" || got.Used != nil || got.Limit != nil || got.Percent != nil || got.Source != "telemetry" {
		t.Fatalf("context = %+v", got)
	}
}
