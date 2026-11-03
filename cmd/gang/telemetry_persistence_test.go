//go:build acceptance

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
)

func TestNativeAuditFailureDoesNotAdvanceCursor(t *testing.T) {
	f := newStateFixture(t)
	f.add(t, "a", "worker", "codex")
	records := []byte(`{"type":"session_meta","payload":{"id":"s"}}
{"timestamp":"2026-09-22T10:00:00Z","type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"total_tokens":40},"model_context_window":200000,"total_token_usage":{"input_tokens":90,"output_tokens":3}}}}
`)
	if err := os.WriteFile(f.transcript, records, 0600); err != nil {
		t.Fatal(err)
	}
	saveAgent(t, f, "a", func(a *core.Agent) {
		a.Native.SessionID, a.Native.Transcript = "s", f.transcript
	})
	p, _ := f.run.team.Agent("a")
	l, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	a, err := p.Read()
	if err != nil {
		t.Fatal(err)
	}
	before := a.Native.Offset
	beforeAgent := a
	backup := filepath.Join(f.env["GANG_STATE_ROOT"], "fixture-audit-before")
	if err := os.Rename(f.run.team.Log, backup); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(f.run.team.Log, 0700); err != nil {
		t.Fatal(err)
	}
	c, err := harness.EmbeddedCollar("codex")
	if err != nil {
		t.Fatal(err)
	}
	if err := f.run.refreshNative(l, &a, c); err == nil {
		t.Fatal("failed audit append was ignored")
	}
	saved, err := p.Read()
	if err != nil || saved.Native.Offset != before || a.Native.Offset != before {
		t.Fatal("source cursor advanced past unrecorded observations")
	}
	if !reflect.DeepEqual(a, beforeAgent) || !reflect.DeepEqual(saved, beforeAgent) {
		t.Fatal("failed audit append changed native or context-band state")
	}
	if err := os.Remove(f.run.team.Log); err != nil { // Empty fixture directory only.
		t.Fatal(err)
	}
	if err := os.Rename(backup, f.run.team.Log); err != nil {
		t.Fatal(err)
	}
	if err := f.run.refreshNative(l, &a, c); err != nil {
		t.Fatal(err)
	}
	if a.Native.Offset != int64(len(records)) {
		t.Fatal("successful retry did not advance to the recorded source boundary")
	}
	data, err := os.ReadFile(f.run.team.Log)
	if err != nil || bytes.Count(data, []byte(`"kind":"token-usage"`)) != 1 {
		t.Fatal("retry lost or duplicated usage")
	}
	if err := f.run.refreshNative(l, &a, c); err != nil {
		t.Fatal(err)
	}
	again, err := os.ReadFile(f.run.team.Log)
	if err != nil || !bytes.Equal(data, again) {
		t.Fatal("unchanged cursor appended another observation")
	}
}

func TestNativeStateFailureReplaysIdentifiableUsage(t *testing.T) {
	f := newStateFixture(t)
	f.add(t, "a", "worker", "codex")
	records := []byte(`{"type":"session_meta","payload":{"id":"s"}}
{"timestamp":"2026-09-22T10:00:00Z","type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"total_tokens":40},"model_context_window":200000,"total_token_usage":{"input_tokens":90,"output_tokens":3}}}}
`)
	if err := os.WriteFile(f.transcript, records, 0600); err != nil {
		t.Fatal(err)
	}
	saveAgent(t, f, "a", func(a *core.Agent) {
		a.Native.SessionID, a.Native.Transcript = "s", f.transcript
	})
	p, _ := f.run.team.Agent("a")
	l, err := p.TryLock()
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	a, err := p.Read()
	if err != nil {
		t.Fatal(err)
	}
	c, err := harness.EmbeddedCollar("codex")
	if err != nil {
		t.Fatal(err)
	}
	backup := filepath.Join(f.env["GANG_STATE_ROOT"], "fixture-state-before")
	if err := os.Rename(p.State, backup); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(p.State, 0700); err != nil {
		t.Fatal(err)
	}
	if err := f.run.refreshNative(l, &a, c); err == nil || a.Native.Offset != 0 {
		t.Fatal("failed state save acknowledged the source in memory")
	}
	if err := os.Remove(p.State); err != nil { // Empty fixture directory only.
		t.Fatal(err)
	}
	if err := os.Rename(backup, p.State); err != nil {
		t.Fatal(err)
	}
	if err := f.run.refreshNative(l, &a, c); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(f.run.team.Log)
	if err != nil {
		t.Fatal(err)
	}
	var usage []core.Reading
	for _, line := range bytes.Split(bytes.TrimSpace(data), []byte("\n")) {
		var e core.Event
		if err := json.Unmarshal(line, &e); err != nil {
			t.Fatal(err)
		}
		for _, r := range e.Readings {
			if r.Kind == "token-usage" {
				usage = append(usage, r)
			}
		}
	}
	if len(usage) != 2 || usage[0].SessionID != "s" || usage[1].SessionID != "s" ||
		usage[0].SourceOffset != int64(len(records)) || usage[1].SourceOffset != usage[0].SourceOffset {
		t.Fatal("state failure lost observations or replay lost its deduplication identity")
	}
}
