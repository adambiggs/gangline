//go:build acceptance

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
)

func TestNativeCompactionStateFailurePreservesPendingCompletion(t *testing.T) {
	for _, status := range []string{"submitted", "unverified"} {
		for _, reload := range []bool{false, true} {
			name := status + "/same-runtime"
			if reload {
				name = status + "/reloaded-runtime"
			}
			t.Run(name, func(t *testing.T) {
				f := newStateFixture(t)
				f.add(t, "a", "worker", "codex")
				records := []byte(`{"type":"session_meta","payload":{"id":"s"}}
{"timestamp":"2026-09-22T10:00:01Z","type":"event_msg","payload":{"type":"context_compacted"}}
`)
				if err := os.WriteFile(f.transcript, records, 0600); err != nil {
					t.Fatal(err)
				}
				saveAgent(t, f, "a", func(a *core.Agent) {
					a.Native.SessionID, a.Native.Transcript = "s", f.transcript
					a.Compaction = &core.Compaction{ID: "c", Status: status, StartedAt: f.cmd.now(), Deadline: f.cmd.now().Add(time.Minute)}
				})
				run := f.run
				l, a, err := run.acquire("a", false)
				if err != nil {
					t.Fatal(err)
				}
				defer func() { _ = l.Close() }()
				before := a
				pending := *a.Compaction // Do not let an alias hide premature mutation.
				before.Compaction = &pending
				stateBytes, err := os.ReadFile(l.Paths.State)
				if err != nil {
					t.Fatal(err)
				}
				c, err := harness.EmbeddedCollar("codex")
				if err != nil {
					t.Fatal(err)
				}
				backup := l.Paths.State + ".before"
				if err := os.Rename(l.Paths.State, backup); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(l.Paths.State, 0700); err != nil {
					t.Fatal(err)
				}
				if err := run.refreshNative(l, &a, c); err == nil {
					t.Fatal("state-save failure was ignored")
				}
				if err := os.Remove(l.Paths.State); err != nil { // Empty fixture directory only.
					t.Fatal(err)
				}
				if err := os.Rename(backup, l.Paths.State); err != nil {
					t.Fatal(err)
				}
				afterBytes, err := os.ReadFile(l.Paths.State)
				if err != nil || !bytes.Equal(stateBytes, afterBytes) {
					t.Fatal("failed save changed retained durable state", err)
				}
				if !reflect.DeepEqual(a, before) {
					t.Fatalf("failed save changed pending completion in memory: before=%v after=%v", before.Compaction.CompletedAt, a.Compaction.CompletedAt)
				}
				if reload {
					if err := l.Close(); err != nil {
						t.Fatal(err)
					}
					run, err = f.cmd.runtime()
					if err != nil {
						t.Fatal(err)
					}
					l, a, err = run.acquire("a", false)
					if err != nil {
						t.Fatal(err)
					}
					if !reflect.DeepEqual(a, before) {
						t.Fatal("reloaded state differs from pre-failure state")
					}
				}
				if err := run.refreshNative(l, &a, c); err != nil {
					t.Fatal(err)
				}
				completed := f.cmd.now().Add(time.Second)
				saved, err := l.Paths.Read()
				if err != nil || saved.Compaction == nil || !saved.Compaction.CompletedAt.Equal(completed) ||
					!a.Compaction.CompletedAt.Equal(completed) || saved.Native.Offset != int64(len(records)) || a.Native.Offset != saved.Native.Offset {
					t.Fatal("retry did not persist compaction completion and cursor together", err)
				}
				data, err := os.ReadFile(run.team.Log)
				if err != nil {
					t.Fatal(err)
				}
				summary := summarizeLogMetadata(metadataPage(t, string(data), 0, int64(len(data))))
				var boundaries []logMetadataBoundary
				for _, event := range summary.Boundaries {
					boundaries = append(boundaries, event.NativeBoundaries...)
				}
				if len(boundaries) != 2 || !reflect.DeepEqual(boundaries[0], boundaries[1]) ||
					boundaries[0].Kind != "compaction-finished" || boundaries[0].SessionID != "s" ||
					boundaries[0].SourceOffset != int64(len(records)) {
					t.Fatalf("failed save replay lost stable compaction identity: %+v", boundaries)
				}
				if err := run.refreshNative(l, &a, c); err != nil {
					t.Fatal(err)
				}
				again, err := os.ReadFile(run.team.Log)
				if err != nil || !bytes.Equal(data, again) {
					t.Fatal("persisted cursor repeated the compaction observation", err)
				}
			})
		}
	}
}

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

func TestNativeSkippedWindowSurvivesStateFailureAndRuntimeReload(t *testing.T) {
	f := newStateFixture(t)
	f.add(t, "a", "worker", "codex")
	header := "{\"type\":\"session_meta\",\"payload\":{\"id\":\"s\"}}\n"
	// One source record larger than the existing tail window has no complete
	// reading in the selected tail. The skipped-window marker must still persist.
	records := header + `{"type":"response_item","body":"` + strings.Repeat("x", 4<<20) + "\"}\n"
	if err := os.WriteFile(f.transcript, []byte(records), 0600); err != nil {
		t.Fatal(err)
	}
	saveAgent(t, f, "a", func(a *core.Agent) { a.Native.SessionID, a.Native.Transcript = "s", f.transcript })
	c, err := harness.EmbeddedCollar("codex")
	if err != nil {
		t.Fatal(err)
	}
	for attempt := range 3 {
		// Reconstruct runtime and reload saved cursor, without touching a host,
		// watchdog, real process or native session.
		run, err := f.cmd.runtime()
		if err != nil {
			t.Fatal(err)
		}
		l, a, err := run.acquire("a", false)
		if err != nil {
			t.Fatal(err)
		}
		backup := l.Paths.State + ".before"
		if attempt == 0 {
			if err := os.Rename(l.Paths.State, backup); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(l.Paths.State, 0700); err != nil {
				t.Fatal(err)
			}
		}
		err = run.refreshNative(l, &a, c)
		closeErr := l.Close()
		if attempt == 0 {
			if err == nil || closeErr != nil || a.Native.Offset != 0 {
				t.Fatal("failed state save acknowledged the skipped source window", err, closeErr)
			}
			if err := os.Remove(l.Paths.State); err != nil { // Empty fixture directory only.
				t.Fatal(err)
			}
			if err := os.Rename(backup, l.Paths.State); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err != nil || closeErr != nil || a.Native.Offset != int64(len(records)) {
			t.Fatal("gap/cursor persistence failed", err, closeErr)
		}
	}
	data, err := os.ReadFile(f.run.team.Log)
	if err != nil {
		t.Fatal(err)
	}
	s := summarizeLogMetadata(metadataPage(t, string(data), 0, int64(len(data))))
	skips := 0
	for _, gap := range s.Gaps {
		if gap.Kind == "source_window_skipped" {
			skips++
		} else if gap.Kind != "observation_time_out_of_order" {
			t.Fatalf("unexpected audit gap: %+v", gap)
		}
	}
	// Failed cursor persistence can replay the same gap, but a successful
	// persisted cursor suppresses further reads even after runtime reload.
	if skips != 2 || bytes.Count(data, []byte("source-window-skipped")) != 2 {
		t.Fatalf("gap lost or repeated across runtime reload: %+v", s.Gaps)
	}
	var boundaries []logMetadataBoundary
	for _, e := range s.Boundaries {
		boundaries = append(boundaries, e.NativeBoundaries...)
	}
	if len(boundaries) != 2 || !reflect.DeepEqual(boundaries[0], boundaries[1]) {
		t.Fatal("replayed gap lost its stable source identity")
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
