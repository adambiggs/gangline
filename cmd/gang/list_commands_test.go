//go:build acceptance

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
)

func TestModelCatalogLimitLeavesFullCatalogAvailable(t *testing.T) {
	f := newStateFixture(t)
	models := make([]map[string]string, 65)
	for i := range models {
		models[i] = map[string]string{"slug": fmt.Sprintf("model-%02d", i)}
	}
	data, err := json.Marshal(map[string]any{"models": models})
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	script := "#!/bin/sh\n# SPDX-License-Identifier: Apache-2.0\nprintf '%s\\n' '" + string(data) + "'\n"
	if err := os.WriteFile(filepath.Join(bin, "codex"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	for _, tc := range []struct {
		args  []string
		count int
	}{{nil, 50}, {[]string{"--limit", "3"}, 3}, {[]string{"--all"}, 65}} {
		f.out.Reset()
		f.errOut.Reset()
		if err := f.cmd.execute(append([]string{"models", "-c", "codex"}, tc.args...)); err != nil {
			t.Fatal(err)
		}
		if strings.Count(f.out.String(), "\n") != tc.count+1 || !strings.HasPrefix(f.out.String(), "model-00\t") {
			t.Fatalf("%v: %q", tc.args, f.out.String())
		}
		if strings.Contains(f.errOut.String(), "--all") != (tc.count < 65) {
			t.Fatalf("notice %q", f.errOut.String())
		}
	}
}

func TestQueueLimitKeepsOldestPendingAndMarksPartialJSON(t *testing.T) {
	f := newStateFixture(t)
	a := f.add(t, "a", "worker", "codex")
	for i := 0; i < 65; i++ {
		publishPending(t, f, a, core.Envelope{ID: core.EnvelopeID(fmt.Sprintf("m-%02d", i)), CreatedAt: f.cmd.now().Add(time.Duration(i-100) * time.Second), Message: core.Message{Text: "pending"}})
	}
	for _, tc := range []struct {
		args  []string
		count int
	}{{nil, 50}, {[]string{"--limit", "3"}, 3}, {[]string{"--all"}, 65}} {
		f.out.Reset()
		f.errOut.Reset()
		if err := f.cmd.execute(append([]string{"queue", "worker", "--json"}, tc.args...)); err != nil {
			t.Fatal(err)
		}
		var got queueJSON
		decodeOutput(t, f, &got)
		if len(got.Messages) != tc.count || got.Total != 65 || got.Truncated != (tc.count < 65) || got.Messages[0].ID != "m-00" {
			t.Fatalf("%v: %+v", tc.args, got)
		}
		if strings.Contains(f.errOut.String(), "--all") != (tc.count < 65) {
			t.Fatalf("notice %q", f.errOut.String())
		}
	}
	f.out.Reset()
	if err := f.cmd.execute([]string{"queue", "--limit", "3"}); err != nil {
		t.Fatal(err)
	}
	if strings.Count(f.out.String(), "\n") != 4 {
		t.Fatalf("table = %q", f.out.String())
	}
}

func TestRosterLimitMarksPartialJSON(t *testing.T) {
	f := newStateFixture(t)
	registerOutsideFixture(t, f)
	for i := 0; i < 60; i++ {
		f.add(t, fmt.Sprintf("h-%02d", i), fmt.Sprintf("worker-%02d", i), "codex")
	}
	for _, tc := range []struct {
		args  []string
		count int
	}{{nil, 50}, {[]string{"--limit", "3"}, 3}, {[]string{"--all"}, 61}} {
		f.out.Reset()
		if err := f.cmd.execute(append([]string{"roster", "--json"}, tc.args...)); err != nil {
			t.Fatal(err)
		}
		var got rosterJSON
		decodeOutput(t, f, &got)
		if len(got.Agents) != tc.count || got.Total != 61 || got.Truncated != (tc.count < 61) {
			t.Fatalf("%v: rows=%d total=%d truncated=%t", tc.args, len(got.Agents), got.Total, got.Truncated)
		}
	}
	f.out.Reset()
	if err := f.cmd.execute([]string{"roster", "--limit", "3"}); err != nil {
		t.Fatal(err)
	}
	if strings.Count(f.out.String(), "\n") != 3 {
		t.Fatalf("table = %q", f.out.String())
	}
}

func TestSnoozeStatusLimitPreservesAllWakeState(t *testing.T) {
	f := newStateFixture(t)
	lead := f.add(t, "lead-id", "lead", "codex")
	saveAgent(t, f, lead.ID, func(a *core.Agent) { a.Role = "lead" })
	f.env["GANG_AGENT_ID"], f.env["TMUX_PANE"] = string(lead.ID), lead.Pane
	if err := f.run.withUsageState(func(state *usageState) error {
		for i := 0; i < 65; i++ {
			id := fmt.Sprintf("worker-%02d", i)
			state.Snoozes[id] = usageSnooze{ID: core.EnvelopeID(fmt.Sprintf("wake-%02d", i)), CallerID: core.HitchID(id), CallerName: core.AgentName(id), At: f.cmd.now().Add(time.Hour)}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		args  []string
		count int
	}{{nil, 50}, {[]string{"--limit", "3"}, 3}, {[]string{"--all"}, 65}} {
		f.out.Reset()
		f.errOut.Reset()
		if err := f.cmd.execute(append([]string{"snooze", "--status"}, tc.args...)); err != nil {
			t.Fatal(err)
		}
		if strings.Count(f.out.String(), "\n") != tc.count || !strings.HasPrefix(f.out.String(), "wake-00\t") {
			t.Fatalf("%v: %q", tc.args, f.out.String())
		}
		if strings.Contains(f.errOut.String(), "--all") != (tc.count < 65) {
			t.Fatalf("notice %q", f.errOut.String())
		}
		if len(usageSnapshot(t, f.run).Snoozes) != 65 {
			t.Fatal("display changed wake state")
		}
	}
}

func TestDiscoveryListsHaveBoundedDefaults(t *testing.T) {
	f := newStateFixture(t)
	config := f.env["GANG_CONFIG_DIR"]
	f.env["GANG_COLLARS"] = filepath.Join(config, "collars")
	for i := 0; i < 65; i++ {
		name := fmt.Sprintf("item-%02d", i)
		team := filepath.Join(f.env["GANG_STATE_ROOT"], "teams", name)
		roles := filepath.Join(config, "roles")
		collars := filepath.Join(config, "collars")
		for _, dir := range []string{team, roles, collars} {
			if err := os.MkdirAll(dir, 0700); err != nil {
				t.Fatal(err)
			}
		}
		for path, body := range map[string]string{filepath.Join(team, "team.json"): "{}", filepath.Join(roles, name+".md"): "A role", filepath.Join(collars, name+".cue"): "collar: {}"} {
			if err := os.WriteFile(path, []byte(body), 0600); err != nil {
				t.Fatal(err)
			}
		}
		if err := f.cmd.execute([]string{"sender", "add", name}); err != nil {
			t.Fatal(err)
		}
	}
	for _, command := range [][]string{{"teams"}, {"roles"}, {"collars"}, {"sender", "list"}} {
		f.out.Reset()
		f.errOut.Reset()
		if err := f.cmd.execute(command); err != nil {
			t.Fatal(err)
		}
		if strings.Count(f.out.String(), "\n") != 50 || !strings.Contains(f.errOut.String(), "--all") {
			t.Fatalf("%v: output=%q warning=%q", command, f.out.String(), f.errOut.String())
		}
		f.out.Reset()
		f.errOut.Reset()
		if err := f.cmd.execute(append(append([]string{}, command...), "--all")); err != nil {
			t.Fatal(err)
		}
		if strings.Count(f.out.String(), "\n") < 65 || f.errOut.Len() != 0 {
			t.Fatalf("full %v: output=%q warning=%q", command, f.out.String(), f.errOut.String())
		}
	}
}
