package main

import (
	"os"
	"strings"
	"testing"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
)

func TestUpPersistsContextBandThresholdsAtHitch(t *testing.T) {
	f := newStateFixture(t)
	fakeCodexOnPath(t)
	f.add(t, "a", "worker", "codex")
	script := "#!/bin/sh\n# SPDX-License-Identifier: Apache-2.0\n" + fakeTmuxUTF8 + "case \"$1 $2\" in\n'list-panes -a') printf '" + strings.Repeat("a", 64) + "\\t$1\\t%%1\\tunit\\n';;\nlist-panes*) printf '%%1\\t" + strings.Repeat("a", 64) + "\\t$1\\tworker\\n';;\nhas-session*) exit 0;;\n*) exit 91;;\nesac\n"
	if err := os.WriteFile(f.env["GANG_TMUX"], []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	if err := f.cmd.up([]string{"--collar", "codex", "--context-bands", "10,20"}); err == nil || !strings.Contains(err.Error(), "spawn pane") {
		t.Fatalf("expected spawn refusal after hitch claim: %v", err)
	}
	agents, err := f.run.team.ListAgents()
	if err != nil {
		t.Fatal(err)
	}
	for _, agent := range agents {
		if agent.Name == "lead" {
			if agent.ContextBandThresholds == nil || agent.ContextBandThresholds.Early != 0.1 || agent.ContextBandThresholds.Late != 0.2 {
				t.Fatalf("lead threshold state = %+v", agent.ContextBandThresholds)
			}
			return
		}
	}
	t.Fatalf("lead hitch record missing after spawn refusal: %+v", agents)
}

func TestContextBandOverrideValidatesPercentagesAndCollar(t *testing.T) {
	c, err := harness.EmbeddedCollar("claude")
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"10", "10,20,30", "-1,20", "20,20", "30,20", "10,101", "NaN,20", "10,+Inf", "10%,20%"} {
		if _, err := contextBandOverride(c, value); err == nil {
			t.Errorf("accepted invalid percentages %q", value)
		}
	}
	invalid := c
	invalid.ContextBands = map[string][]harness.ContextBand{"*": {{Name: "only", At: 0.5}}}
	if _, err := contextBandOverride(invalid, "10,20"); err == nil || !strings.Contains(err.Error(), `selector "*"`) {
		t.Fatalf("accepted incompatible collar: %v", err)
	}
	if thresholds, err := contextBandOverride(c, ""); err != nil || thresholds != nil {
		t.Fatalf("default override = %+v, %v", thresholds, err)
	}
}

func TestContextBandOverrideSurvivesStateAndCollarChanges(t *testing.T) {
	c, err := harness.EmbeddedCollar("claude")
	if err != nil {
		t.Fatal(err)
	}
	thresholds, err := contextBandOverride(c, "10,20")
	if err != nil {
		t.Fatal(err)
	}
	f := newStateFixture(t)
	a := f.add(t, "a", "lead", "claude")
	a.ContextBandThresholds = thresholds
	p, err := f.run.team.Agent(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	l, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Save(a); err != nil {
		t.Fatal(err)
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	a, err = p.Read()
	if err != nil {
		t.Fatal(err)
	}
	c.ContextBands["*"][0].At = 0.7
	c.ContextBands["*"][1].At = 0.9
	c.ContextBands["*"][0].Message = "edited {{threshold_percent}}%"
	c.ContextBands["*sonnet*"] = []harness.ContextBand{{Name: "new-early", At: 0.3}, {Name: "new-late", At: 0.6}}
	c.ContextBands["*new*"] = []harness.ContextBand{{Name: "first", At: 0.2}, {Name: "middle", At: 0.5}, {Name: "last", At: 0.8}}
	selected := agentContextCollar(a, c)
	for _, model := range []string{"claude-opus-test", "claude-haiku-test", "claude-sonnet-test"} {
		bands := harness.CrossedContextBands(selected, model, -1, 0.2)
		if len(bands) != 2 || bands[0].At != 0.1 || bands[1].At != 0.2 {
			t.Errorf("%s bands = %+v", model, bands)
		}
	}
	if selected.ContextBands["*"][0].Message != "edited {{threshold_percent}}%" || selected.ContextBands["*sonnet*"][0].Name != "new-early" {
		t.Fatalf("live collar definitions lost: %+v", selected.ContextBands)
	}
	if bands := selected.ContextBands["*new*"]; len(bands) != 3 || bands[0].At != 0.2 || bands[1].At != 0.5 || bands[2].At != 0.8 {
		t.Fatalf("incompatible later selector changed: %+v", bands)
	}
	if band := harness.ActiveContextBand(c, "claude-opus-test", harness.ContextReading{Percent: 0.2}); band != nil {
		t.Fatalf("collar changed with override: %+v", band)
	}
	if got := agentContextCollar(core.Agent{}, c); got.ContextBands["*"][0].At != 0.7 {
		t.Fatalf("agent without override lost current collar: %+v", got.ContextBands["*"])
	}
	used, limit, percent := int64(1500), int64(10000), 15.0
	a.Native.Context = core.Reading{Status: "observed", Model: "claude-opus-test", Used: &used, Limit: &limit, Percent: &percent}
	if err := f.run.noteContextBands(&a, c); err != nil {
		t.Fatal(err)
	}
	if len(a.ContextBands.Pending) != 1 || a.ContextBands.Pending[0].Band != "early" || a.ContextBands.Pending[0].Envelope.Message.Text != "edited 10%" {
		t.Fatalf("hitch bands did not govern notice: %+v", a.ContextBands.Pending)
	}
}

func TestContextCommandUsesHitchedBandThresholds(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "codex")
	c, err := harness.EmbeddedCollar("codex")
	if err != nil {
		t.Fatal(err)
	}
	thresholds, err := contextBandOverride(c, "10,20")
	if err != nil {
		t.Fatal(err)
	}
	used, limit, percent := int64(1500), int64(10000), 15.0
	saveAgent(t, f, a.ID, func(a *core.Agent) {
		a.ContextBandThresholds = thresholds
		a.Native.Context = core.Reading{Kind: "context", Source: "fixture", Model: "gpt-test", Status: "observed", Used: &used, Limit: &limit, Percent: &percent}
	})
	if err := f.cmd.execute([]string{"context", "worker", "--json"}); err != nil {
		t.Fatal(err)
	}
	var got contextJSON
	decodeOutput(t, f, &got)
	if got.Band == nil || *got.Band != "yellow" {
		t.Fatalf("context did not use hitched thresholds: %+v", got)
	}
}
