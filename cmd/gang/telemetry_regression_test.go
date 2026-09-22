package main

import (
	"bytes"
	"fmt"
	"github.com/adambiggs/gangline/core"
	"github.com/adambiggs/gangline/harness"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStatuslineRendersStructuredContextWithoutTeam(t *testing.T) {
	var out bytes.Buffer
	cmd := command{stdin: strings.NewReader(`{"session_id":"s1","model":{"id":"claude-test"},"context_window":{"context_window_size":200000,"current_usage":{"input_tokens":1000,"cache_creation_input_tokens":2000,"cache_read_input_tokens":3000,"output_tokens":500}}}`), stdout: &out, stderr: &out, getenv: func(string) string { return "" }}
	if err := cmd.execute([]string{"statusline"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "6000/200000") {
		t.Fatalf("context output = %q", out.String())
	}
}

func TestActivityImportsNativeContextAndAutomaticCompaction(t *testing.T) {
	cmd, run := hookFixture(t, "")
	path := filepath.Join(t.TempDir(), "native.jsonl")
	data := `{"timestamp":"2026-09-22T10:00:00Z","type":"session_meta","payload":{"id":"s1"}}
{"timestamp":"2099-09-22T10:00:01Z","type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":100,"output_tokens":20,"total_tokens":120},"model_context_window":200000},"rate_limits":null}}
{"timestamp":"2099-09-22T10:00:02Z","type":"event_msg","payload":{"type":"context_compacted"}}
`
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	cmd.stdin = strings.NewReader(fmt.Sprintf(`{"hook_event_name":"PostToolUse","session_id":"s1","transcript_path":%q}`, path))
	if err := cmd.hook(nil); err != nil {
		t.Fatal(err)
	}
	paths, err := run.paths().Team(run.settings.Session)
	if err != nil {
		t.Fatal(err)
	}
	log, err := os.ReadFile(paths.Events)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{`"kind":"context"`, `"kind":"compaction-finished"`, `"source":"session-log"`} {
		if !bytes.Contains(log, []byte(expected)) {
			t.Errorf("missing %s in log", expected)
		}
	}
}

func TestLogFiltersByAgentAndType(t *testing.T) {
	cmd, _ := hookFixture(t, `{"hook_event_name":"PostToolUse"}`)
	if err := cmd.hook(nil); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	cmd.stdout = &out
	if err := cmd.log([]string{"--agent", "worker", "--type", "native_hook"}); err != nil {
		t.Fatal(err)
	}
	if strings.Count(out.String(), `"type":"native_hook"`) != 2 || strings.Contains(out.String(), `"type":"adopt_requested"`) {
		t.Fatalf("filtered output = %s", out.String())
	}
}

func TestStartupTrustLeavesObservableEvidence(t *testing.T) {
	event := bootObservationOutcome(time.Now(), core.AwaitBoot{HitchID: "h1"}, harness.Startup{State: harness.StartupTrustRequired, Prompt: "Hooks need review"}, nil)
	observation, ok := event.(core.Observation)
	if !ok || len(observation.Readings) != 1 || observation.Readings[0].Kind != "blocked" {
		t.Fatalf("trust prompt is not recorded: %#v", event)
	}
}

func TestObservationCursorIsCommittedAndDoesNotReplay(t *testing.T) {
	cmd, run := hookFixture(t, "")
	path := filepath.Join(t.TempDir(), "native.jsonl")
	data := `{"type":"session_meta","payload":{"id":"s1"}}
{"timestamp":"2099-09-22T10:00:00Z","type":"turn_context","payload":{"model":"gpt-test"}}
{"timestamp":"2099-09-22T10:00:01Z","type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"total_tokens":120},"model_context_window":200000}}}
`
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		cmd.stdin = strings.NewReader(fmt.Sprintf(`{"hook_event_name":"PostToolUse","session_id":"s1","transcript_path":%q}`, path))
		if err := cmd.hook(nil); err != nil {
			t.Fatal(err)
		}
	}
	paths, err := run.paths().Team(run.settings.Session)
	if err != nil {
		t.Fatal(err)
	}
	log, err := os.ReadFile(paths.Events)
	if err != nil {
		t.Fatal(err)
	}
	if count := bytes.Count(log, []byte(`"native_event":"token_count"`)); count != 1 {
		t.Fatalf("native reading recorded %d times", count)
	}
	latest, err := run.latestReadings("h-1")
	if err != nil {
		t.Fatal(err)
	}
	if latest.Offset != int64(len(data)) || latest.Context.Model != "gpt-test" || latest.Context.Used == nil || *latest.Context.Used != 120 {
		t.Fatalf("latest = %#v", latest)
	}
	latestPath, err := run.latestPath("h-1")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(latestPath); err != nil {
		t.Fatal(err)
	}
	rebuilt, err := run.latestReadings("h-1")
	if err != nil {
		t.Fatal(err)
	}
	if rebuilt.Context.Used == nil || *rebuilt.Context.Used != 120 {
		t.Fatal("deleted derivative lost committed context")
	}
}

func TestFilteredReplayJoinsDeliveryAndCompactionOutcomes(t *testing.T) {
	now := time.Now()
	deadline := now.Add(time.Minute)
	events := []core.Event{
		core.AdoptRequested{At: now, Hitch: core.Hitch{ID: "h1", Name: "worker", Collar: "codex", Directory: "/work"}, Pane: "%1"},
		core.SendRequested{At: now, Envelope: core.Envelope{ID: "e1", From: core.Sender{Kind: core.SenderSelfDeclared, Name: "operator"}, To: "worker", Message: core.Message{Text: "hi"}, CreatedAt: now}, Deadline: deadline},
		core.DeliverySucceeded{At: now, EnvelopeID: "e1"},
		core.CompactionRequested{At: now, Compaction: core.Compaction{ID: "c1", HitchID: "h1", Resume: core.Message{Text: "continue"}, Deadline: deadline}},
		core.CompactionCompleted{At: now, CompactionID: "c1"},
		core.OperationTimedOut{At: now, Operation: core.TimeoutWait, ID: "h1", Deadline: now, Evidence: "idle boundary not observed"},
	}
	var input bytes.Buffer
	for _, event := range events {
		data, err := core.EncodeEvent(event)
		if err != nil {
			t.Fatal(err)
		}
		fmt.Fprintln(&input, string(data))
	}
	for _, kind := range []string{"delivery_succeeded", "compaction_completed", "operation_timed_out"} {
		var out bytes.Buffer
		cmd := command{stdin: strings.NewReader(input.String()), stdout: &out}
		if err := cmd.replay([]string{"--agent", "worker", "--type", kind}); err != nil {
			t.Fatal(err)
		}
		if strings.Count(out.String(), `"type":"`+kind+`"`) != 1 {
			t.Fatalf("missing outcome %s: %s", kind, out.String())
		}
	}
}

func TestCompactionDoesNotAcceptDelayedStatuslineAsFresh(t *testing.T) {
	_, run := hookFixture(t, "")
	used, limit, percent := int64(100), int64(200000), 0.05
	before := time.Now().Add(-time.Minute)
	old := harness.Reading{Kind: "context", Source: "status-line", Status: "observed", At: &before, Used: &used, Limit: &limit, Percent: &percent}
	if err := run.recordStatusline("h-1", "s", []harness.Reading{old}); err != nil {
		t.Fatal(err)
	}
	compactAt := time.Now()
	locked, err := run.lock()
	if err != nil {
		t.Fatal(err)
	}
	err = locked.Append(core.Observation{At: compactAt, HitchID: "h-1", Collar: "codex", SessionID: "s", Readings: []core.Reading{{Kind: "compaction-finished", Source: "native-hook", Status: "observed"}}})
	closeErr := locked.Close()
	if err != nil || closeErr != nil {
		t.Fatalf("append: %v, close: %v", err, closeErr)
	}
	if err := run.recordStatusline("h-1", "s", []harness.Reading{old}); err != nil {
		t.Fatal(err)
	}
	latest, err := run.latestReadings("h-1")
	if err != nil {
		t.Fatal(err)
	}
	if latest.Context.Status != "unknown" {
		t.Fatalf("late pre-compaction context became fresh: %#v", latest.Context)
	}
	after := compactAt.Add(time.Second)
	fresh := old
	fresh.At = &after
	if err := run.recordStatusline("h-1", "s", []harness.Reading{fresh}); err != nil {
		t.Fatal(err)
	}
	latest, err = run.latestReadings("h-1")
	if err != nil {
		t.Fatal(err)
	}
	if latest.Context.Status != "observed" || latest.Context.At == nil || !latest.Context.At.Equal(after) {
		t.Fatalf("new measurement not accepted: %#v", latest.Context)
	}
}

func TestMalformedStatuslineLeavesFailureEvidence(t *testing.T) {
	cmd, run := hookFixture(t, `{"bad":"payload"}`)
	if err := cmd.statusline(nil); err == nil {
		t.Fatal("invalid status line accepted")
	}
	records := hookRecords(t, run)
	if len(records) != 1 || records[0]["native_event"] != "statusline" || records[0]["status"] != "failed" {
		t.Fatalf("failure = %#v", records)
	}
}

func TestObservationErrorDoesNotSuppressWitnessOrTurnBoundary(t *testing.T) {
	cmd, run := hookFixture(t, "")
	oldEnv := cmd.getenv
	socket := filepath.Join(t.TempDir(), "absent-private.sock")
	cmd.getenv = func(key string) string {
		if key == "GANG_TMUX_SOCKET" {
			return socket
		}
		return oldEnv(key)
	}
	run.cmd = cmd
	run.settings.Socket = socket
	native := filepath.Join(t.TempDir(), "malformed.jsonl")
	if err := os.WriteFile(native, []byte("{\"type\":\"session_meta\",\"payload\":{\"id\":\"s\"}}\n{bad}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	// An immediately openable receipt sink isolates observation ordering from
	// FIFO scheduling, whose transport is covered by the acceptance suite.
	witness := run.deliveryWitnessPath("h-1")
	if err := os.WriteFile(witness, nil, 0600); err != nil {
		t.Fatal(err)
	}
	cmd.stdin = strings.NewReader(fmt.Sprintf(`{"hook_event_name":"UserPromptSubmit","session_id":"s","transcript_path":%q,"prompt":"exact native receipt"}`, native))
	if err := cmd.hook(nil); err == nil || !strings.Contains(err.Error(), "decode session log") {
		t.Fatalf("observation error = %v", err)
	}
	got, err := os.ReadFile(witness)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "exact native receipt" {
		t.Fatalf("witness suppressed: %q", got)
	}
	state, err := run.load()
	if err != nil {
		t.Fatal(err)
	}
	if state.Hitches["h-1"].Activity != core.ActivityBusy {
		t.Fatalf("turn start suppressed: %#v", state.Hitches["h-1"])
	}
	cmd.stdin = strings.NewReader(fmt.Sprintf(`{"hook_event_name":"Stop","session_id":"s","transcript_path":%q}`, native))
	if err := cmd.hook(nil); err == nil {
		t.Fatal("observation failure was swallowed")
	}
	state, err = run.load()
	if err != nil {
		t.Fatal(err)
	}
	if state.Hitches["h-1"].Activity != core.ActivityIdle {
		t.Fatalf("turn end suppressed: %#v", state.Hitches["h-1"])
	}
}

func TestManagedStatuslineRendersAcceptedUnknownAfterCompaction(t *testing.T) {
	cmd, run := hookFixture(t, `{"session_id":"s","context_window":{"context_window_size":200000,"current_usage":{"input_tokens":100,"cache_creation_input_tokens":0,"cache_read_input_tokens":0}}}`)
	locked, err := run.lock()
	if err != nil {
		t.Fatal(err)
	}
	err = locked.Append(core.Observation{At: time.Now(), HitchID: "h-1", Collar: "codex", SessionID: "s", Readings: []core.Reading{{Kind: "compaction-finished", Source: "native-hook", Status: "observed"}}})
	closeErr := locked.Close()
	if err != nil || closeErr != nil {
		t.Fatalf("append=%v close=%v", err, closeErr)
	}
	var out bytes.Buffer
	cmd.stdout = &out
	if err := cmd.statusline(nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "unknown") || strings.Contains(out.String(), "100/200000") {
		t.Fatalf("renderer bypassed accepted unknown: %q", out.String())
	}
}
