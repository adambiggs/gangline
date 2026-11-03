//go:build acceptance

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
)

const replacementHeader = "{\"type\":\"session_meta\",\"payload\":{\"id\":\"s\"}}\n"
const replacementRecord = "{\"timestamp\":\"2026-09-22T10:00:00Z\",\"type\":\"event_msg\",\"payload\":{\"type\":\"token_count\",\"info\":{\"last_token_usage\":{\"total_tokens\":40},\"model_context_window\":200000,\"total_token_usage\":{\"input_tokens\":90,\"output_tokens\":3}}}}\n"

func TestNativeReplacementContextGap(t *testing.T) {
	for _, mode := range []string{"rename-same-length", "rename-regrown", "inplace-same-length", "inplace-regrown", "append", "unchanged-context"} {
		t.Run(mode, func(t *testing.T) {
			f := newStateFixture(t)
			f.add(t, "a", "worker", "codex")
			old := []byte(replacementHeader + replacementRecord)
			if err := os.WriteFile(f.transcript, old, 0600); err != nil {
				t.Fatal(err)
			}
			saveAgent(t, f, "a", func(a *core.Agent) { a.Native.SessionID, a.Native.Transcript = "s", f.transcript })
			c, err := harness.EmbeddedCollar("codex")
			if err != nil {
				t.Fatal(err)
			}
			refresh := func() core.Agent {
				run, err := f.cmd.runtime()
				if err != nil {
					t.Fatal(err)
				}
				l, a, err := run.acquire("a", false)
				if err != nil {
					t.Fatal(err)
				}
				defer l.Close()
				if err := run.refreshNative(l, &a, c); err != nil {
					t.Fatal(err)
				}
				return a
			}
			refresh()
			replacement := []byte(strings.ReplaceAll(string(old), `"total_tokens":40`, `"total_tokens":60`))
			later := strings.ReplaceAll(strings.ReplaceAll(replacementRecord, `"total_tokens":40`, `"total_tokens":70`), "10:00:00Z", "10:00:01Z")
			if strings.Contains(mode, "regrown") {
				replacement = append(replacement, []byte(later)...)
			}
			switch mode {
			case "append":
				replacement = append(append([]byte{}, old...), []byte(later)...)
			case "unchanged-context":
				replacement = bytes.ReplaceAll(old, []byte(`"input_tokens":90`), []byte(`"input_tokens":20`))
			}
			if strings.HasPrefix(mode, "rename") {
				if err := os.WriteFile(f.transcript+".new", replacement, 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.Rename(f.transcript+".new", f.transcript); err != nil {
					t.Fatal(err)
				}
			} else if mode == "append" {
				file, err := os.OpenFile(f.transcript, os.O_WRONLY|os.O_APPEND, 0)
				if err != nil {
					t.Fatal(err)
				}
				_, err = file.WriteString(later)
				closeErr := file.Close()
				if err != nil || closeErr != nil {
					t.Fatal(err, closeErr)
				}
			} else if err := os.WriteFile(f.transcript, replacement, 0600); err != nil {
				t.Fatal(err)
			}
			a := refresh()
			data, err := os.ReadFile(f.run.team.Log)
			if err != nil {
				t.Fatal(err)
			}
			want := 1
			if mode == "append" || mode == "unchanged-context" {
				want = 0
			}
			if got := bytes.Count(data, []byte(`"native_event":"source-context-mismatch"`)); got != want {
				t.Fatalf("missing/misclassified context mismatch: got %d want %d", got, want)
			}
			if strings.Contains(mode, "same-length") && (a.Native.Context.Status != "unknown" || a.Native.Context.Used != nil) {
				t.Fatal("mismatched saved context stayed observed")
			}
			if strings.Contains(mode, "regrown") && (a.Native.Context.Used == nil || *a.Native.Context.Used != 70) {
				t.Fatal("suffix context missing")
			}
			refresh()
			again, err := os.ReadFile(f.run.team.Log)
			if err != nil || !bytes.Equal(data, again) {
				t.Fatal("settled mismatch repeated", err)
			}
			summary := summarizeLogMetadata(metadataPage(t, string(data), 0, int64(len(data))))
			count := 0
			for _, gap := range summary.Gaps {
				if gap.Kind == "source_context_mismatch" {
					count++
				}
			}
			if count != want {
				t.Fatalf("summary lost mismatch gap: %+v", summary.Gaps)
			}
		})
	}
}

func TestNativeReplacementGapPersistenceFailures(t *testing.T) {
	for _, failure := range []string{"audit", "state"} {
		t.Run(failure, func(t *testing.T) {
			f := newStateFixture(t)
			f.add(t, "a", "worker", "codex")
			old := []byte(replacementHeader + replacementRecord)
			if err := os.WriteFile(f.transcript, old, 0600); err != nil {
				t.Fatal(err)
			}
			saveAgent(t, f, "a", func(a *core.Agent) { a.Native.SessionID, a.Native.Transcript = "s", f.transcript })
			c, err := harness.EmbeddedCollar("codex")
			if err != nil {
				t.Fatal(err)
			}
			l, a, err := f.run.acquire("a", false)
			if err != nil {
				t.Fatal(err)
			}
			if err := f.run.refreshNative(l, &a, c); err != nil {
				t.Fatal(err)
			}
			before := a.Native
			if before.Context.SessionID != "s" || before.Context.SourceOffset != int64(len(old)) {
				t.Fatal("missing persisted context binding")
			}
			data, err := os.ReadFile(f.run.team.Log)
			if err != nil {
				t.Fatal(err)
			}
			// Session/offset metadata is retained in agent state, not duplicated on
			// every historical context record; paired token usage keeps its identity.
			for _, line := range bytes.Split(bytes.TrimSpace(data), []byte("\n")) {
				var e core.Event
				if err := json.Unmarshal(line, &e); err != nil {
					t.Fatal(err)
				}
				for _, r := range e.Readings {
					if r.Kind == "context" && (r.SessionID != "" || r.SourceOffset != 0) {
						t.Fatal("context identity grew the audit")
					}
				}
			}
			replacement := bytes.ReplaceAll(old, []byte(`"total_tokens":40`), []byte(`"total_tokens":60`))
			if err := os.WriteFile(f.transcript, replacement, 0600); err != nil {
				t.Fatal(err)
			}
			path := f.run.team.Log
			if failure == "state" {
				path = l.Paths.State
			}
			backup := path + ".before"
			if err := os.Rename(path, backup); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(path, 0700); err != nil {
				t.Fatal(err)
			}
			if err := f.run.refreshNative(l, &a, c); err == nil {
				t.Fatal("expected failed persistence")
			}
			if !reflect.DeepEqual(a.Native, before) {
				t.Fatal("failed persistence acknowledged mismatch in memory")
			}
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if err := os.Rename(backup, path); err != nil {
				t.Fatal(err)
			}
			if err := l.Close(); err != nil {
				t.Fatal(err)
			}
			for range 2 {
				run, err := f.cmd.runtime()
				if err != nil {
					t.Fatal(err)
				}
				l, a, err := run.acquire("a", false)
				if err != nil {
					t.Fatal(err)
				}
				refreshErr := run.refreshNative(l, &a, c)
				closeErr := l.Close()
				if refreshErr != nil || closeErr != nil {
					t.Fatal(refreshErr, closeErr)
				}
				if a.Native.Context.Status != "unknown" || a.Native.Offset != int64(len(old)) {
					t.Fatal("mismatch reset/cursor not persisted")
				}
			}
			data, err = os.ReadFile(f.run.team.Log)
			if err != nil {
				t.Fatal(err)
			}
			want := 1
			if failure == "state" {
				want = 2
			}
			if got := bytes.Count(data, []byte(`"native_event":"source-context-mismatch"`)); got != want {
				t.Fatalf("gap retry count %d want %d", got, want)
			}
		})
	}
}

func TestNativeLegacyContextRemainsUnboundUntilNewReading(t *testing.T) {
	f := newStateFixture(t)
	f.add(t, "a", "worker", "codex")
	old := []byte(replacementHeader + replacementRecord)
	if err := os.WriteFile(f.transcript, old, 0600); err != nil {
		t.Fatal(err)
	}
	saveAgent(t, f, "a", func(a *core.Agent) { a.Native.SessionID, a.Native.Transcript = "s", f.transcript })
	c, err := harness.EmbeddedCollar("codex")
	if err != nil {
		t.Fatal(err)
	}
	l, a, err := f.run.acquire("a", false)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	if err := f.run.refreshNative(l, &a, c); err != nil {
		t.Fatal(err)
	}
	a.Native.Context.SessionID, a.Native.Context.SourceOffset = "", 0
	if err := l.Save(a); err != nil {
		t.Fatal(err)
	}
	replacement := bytes.ReplaceAll(old, []byte(`"total_tokens":40`), []byte(`"total_tokens":60`))
	if err := os.WriteFile(f.transcript, replacement, 0600); err != nil {
		t.Fatal(err)
	}
	if err := f.run.refreshNative(l, &a, c); err != nil {
		t.Fatal(err)
	}
	if a.Native.Context.SourceOffset != 0 || a.Native.Context.SessionID != "" {
		t.Fatal("legacy witness falsely bound to current file")
	}
	data, err := os.ReadFile(f.run.team.Log)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte("source-context-mismatch")) {
		t.Fatal("legacy witness asserted a mismatch")
	}
	next := strings.ReplaceAll(replacementRecord, "10:00:00Z", "10:00:01Z")
	file, err := os.OpenFile(f.transcript, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, writeErr := file.WriteString(next)
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil {
		t.Fatal(writeErr, closeErr)
	}
	if err := f.run.refreshNative(l, &a, c); err != nil {
		t.Fatal(err)
	}
	if a.Native.Context.SourceOffset != int64(len(replacement)+len(next)) || a.Native.Context.SessionID != "s" {
		t.Fatal("new context did not establish a bound witness")
	}
}
