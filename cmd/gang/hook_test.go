package main

import (
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/store"
)

type hookPayload struct {
	io.Reader
	before func()
}

func (p *hookPayload) Read(b []byte) (int, error) {
	if p.before != nil {
		f := p.before
		p.before = nil
		f()
	}
	return p.Reader.Read(b)
}

func hookFixture(t *testing.T, payload string) (command, *runtime) {
	t.Helper()
	root := t.TempDir()
	values := map[string]string{"GANG_SESSION": "hook-test", "GANG_STATE_ROOT": root, "XDG_CONFIG_HOME": root, "GANGLINE_HITCH_ID": "h-1"}
	cmd := command{stdin: strings.NewReader(payload), stdout: io.Discard, stderr: io.Discard, getenv: func(k string) string { return values[k] }, userHomeDir: func() (string, error) { return root, nil }}
	run, err := cmd.runtime()
	if err != nil {
		t.Fatal(err)
	}
	locked, err := run.paths().Lock(run.settings.Session)
	if err != nil {
		t.Fatal(err)
	}
	if err = locked.Append(core.AdoptRequested{At: time.Now(), Pane: "%1", Hitch: core.Hitch{ID: "h-1", Name: "worker", Collar: "codex", Directory: root}}); err != nil {
		t.Fatal(err)
	}
	if err = locked.Close(); err != nil {
		t.Fatal(err)
	}
	return cmd, run
}
func hookRecords(t *testing.T, run *runtime) []map[string]any {
	t.Helper()
	paths, err := run.paths().Team(run.settings.Session)
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(paths.Events)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	entries, err := store.ReadLog(f)
	if err != nil {
		t.Fatal(err)
	}
	var records []map[string]any
	for _, entry := range entries {
		b, err := core.EncodeEvent(entry.Event)
		if err != nil {
			t.Fatal(err)
		}
		var r map[string]any
		if err = json.Unmarshal(b, &r); err != nil {
			t.Fatal(err)
		}
		if r["type"] == "native_hook" {
			records = append(records, r)
		}
	}
	return records
}
func TestHookReadsNativeInputBeforeContendedStore(t *testing.T) {
	cmd, run := hookFixture(t, `{"hook_event_name":"PostToolUse"}`)
	locked, err := run.paths().Lock(run.settings.Session)
	if err != nil {
		t.Fatal(err)
	}
	defer locked.Close()
	cmd.stdin = &hookPayload{Reader: cmd.stdin, before: func() {
		if err := locked.Close(); err != nil {
			t.Fatal(err)
		}
	}}
	if err := cmd.hook(nil); err != nil {
		t.Fatalf("normal native activity refused: %v", err)
	}
	records := hookRecords(t, run)
	if len(records) != 2 || records[0]["status"] != "received" || records[1]["status"] != "completed" {
		t.Fatalf("native activity missing from log: %#v", records)
	}
}
func TestHookDecodeFailureIsRecorded(t *testing.T) {
	cmd, run := hookFixture(t, `{"hook_event_name":"FutureNativeEvent"}`)
	if err := cmd.hook(nil); err == nil {
		t.Fatal("unknown native event was swallowed")
	}
	records := hookRecords(t, run)
	if len(records) != 2 || records[1]["status"] != "failed" || !strings.Contains(records[1]["reason"].(string), "FutureNativeEvent") {
		t.Fatalf("native failure missing from log: %#v", records)
	}
}
func TestHookAfterDropIsAnObservedNoop(t *testing.T) {
	cmd, run := hookFixture(t, `{"hook_event_name":"Stop"}`)
	locked, err := run.paths().Lock(run.settings.Session)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range []core.Event{core.DropRequested{At: time.Now(), HitchID: "h-1", Deadline: time.Now().Add(time.Minute)}, core.DropSucceeded{At: time.Now(), HitchID: "h-1"}} {
		if err := locked.Append(e); err != nil {
			t.Fatal(err)
		}
	}
	if err := locked.Close(); err != nil {
		t.Fatal(err)
	}
	if err := cmd.hook(nil); err != nil {
		t.Fatalf("late dropped hook refused: %v", err)
	}
	records := hookRecords(t, run)
	if len(records) != 2 || records[1]["status"] != "ignored" {
		t.Fatalf("late hook outcome = %#v", records)
	}
}

func TestConcurrentActivityHooksAllRecordOutcomes(t *testing.T) {
	cmd, run := hookFixture(t, "")
	start := make(chan struct{})
	results := make(chan error, 8)
	for i := 0; i < 8; i++ {
		go func() {
			own := cmd
			own.stdin = strings.NewReader(`{"hook_event_name":"PostToolUse"}`)
			<-start
			results <- own.hook(nil)
		}()
	}
	close(start)
	for i := 0; i < 8; i++ {
		if err := <-results; err != nil {
			t.Errorf("normal concurrent hook failed: %v", err)
		}
	}
	records := hookRecords(t, run)
	outcomes := map[string]int{}
	for _, r := range records {
		if r["status"] == "completed" {
			outcomes[r["id"].(string)]++
		}
	}
	if len(records) != 16 || len(outcomes) != 8 {
		t.Fatalf("hook receipts/outcomes missing: %d records, %v", len(records), outcomes)
	}
	for id, n := range outcomes {
		if n != 1 {
			t.Errorf("hook %s completed %d times", id, n)
		}
	}
}
