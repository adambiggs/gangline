package harness

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// claudeTranscript writes Claude Code transcript records in the order given;
// each record's timestamp is its offset in seconds from a fixed start.
func claudeTranscript(t *testing.T, records ...string) string {
	t.Helper()
	start := time.Date(2026, 10, 2, 10, 55, 0, 0, time.UTC)
	var lines []string
	for _, r := range records {
		var at float64
		var kind, value string
		if _, err := fmt.Sscanf(r, "%g %s %s", &at, &kind, &value); err != nil {
			t.Fatalf("record %q: %v", r, err)
		}
		stamp := start.Add(time.Duration(at * float64(time.Second))).Format(time.RFC3339Nano)
		switch kind {
		case "user":
			lines = append(lines, fmt.Sprintf(`{"type":"user","promptId":%q,"timestamp":%q,"message":{"role":"user","content":"x"}}`, value, stamp))
		case "queue":
			lines = append(lines, fmt.Sprintf(`{"type":"queue-operation","operation":%q,"timestamp":%q,"content":"next"}`, value, stamp))
		case "system":
			lines = append(lines, fmt.Sprintf(`{"type":"system","subtype":%q,"timestamp":%q}`, value, stamp))
		case "assistant":
			lines = append(lines, fmt.Sprintf(`{"type":"assistant","timestamp":%q,"message":{"role":"assistant","content":[]}}`, stamp))
		default:
			t.Fatalf("record kind %q", kind)
		}
	}
	path := filepath.Join(t.TempDir(), "transcript.jsonl")
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

var claudeQueuedTurns = Invocation{Name: "hook-boundary", Params: map[string]string{"queued_turns": "claude-transcript"}}

func TestQueuedTurnPending(t *testing.T) {
	for _, test := range []struct {
		name    string
		turn    string
		records []string
		want    bool
	}{
		{"prompt queued behind the turn", "p1", []string{"0 user p1", "1 queue enqueue", "2 assistant -"}, true},
		{"no prompt queued", "p1", []string{"0 user p1", "2 assistant -", "3 system stop_hook_summary"}, false},
		// Claude Code writes the dequeue ahead of the turn's last records.
		{"dequeue written before the hook reads", "p1", []string{"0 user p1", "1 queue enqueue", "4 queue dequeue", "2 assistant -", "3 system stop_hook_summary"}, true},
		{"the dequeued turn's own finish", "p2", []string{"0 user p1", "1 queue enqueue", "4 queue dequeue", "2 assistant -", "3 system stop_hook_summary", "5 user p2", "6 assistant -"}, false},
		{"prompt absorbed into the running turn", "p1", []string{"0 user p1", "1 queue enqueue", "2 queue remove", "3 assistant -"}, false},
		{"queue cleared", "p1", []string{"0 user p1", "1 queue enqueue", "2 queue popAll"}, false},
		{"a later prompt after a turn end empties an unrecorded queue", "p1", []string{"0 user p0", "1 queue enqueue", "2 system stop_hook_summary", "3 user p1", "4 assistant -"}, false},
		{"turn_duration also ends a turn", "p1", []string{"0 user p0", "1 queue enqueue", "2 system turn_duration", "3 user p1", "4 assistant -"}, false},
		{"a continued turn keeps its queue", "p1", []string{"0 user p1", "1 queue enqueue", "2 system stop_hook_summary", "3 user p1", "4 assistant -"}, true},
		{"second queued prompt waits behind the first", "p2", []string{"0 user p1", "1 queue enqueue", "1.5 queue enqueue", "3 queue dequeue", "2 system stop_hook_summary", "4 user p2", "5 assistant -"}, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := QueuedTurnPending(claudeQueuedTurns, claudeTranscript(t, test.records...), test.turn)
			if err != nil || got != test.want {
				t.Fatalf("pending=%v err=%v, want %v", got, err, test.want)
			}
		})
	}
}

func TestTurnRanAfter(t *testing.T) {
	// A turn writes many user records, tool results among them, and Claude
	// Code writes them out of time order; the first record dates the prompt.
	transcript := claudeTranscript(t, "0 user p1", "1 queue enqueue", "4 user p2", "3.5 user p2", "4.5 user p1", "5 user p0")
	for _, test := range []struct {
		earlier, later string
		want           bool
	}{
		{"p1", "p2", true},
		{"p2", "p1", false},
		{"p1", "p1", false},
		{"p2", "p0", true},
		{"p1", "missing", false},
		{"missing", "p2", false},
	} {
		got, err := TurnRanAfter(claudeQueuedTurns, transcript, test.earlier, test.later)
		if err != nil || got != test.want {
			t.Fatalf("%s before %s = %v err=%v, want %v", test.earlier, test.later, got, err, test.want)
		}
	}
	if got, err := TurnRanAfter(Invocation{Name: "hook-boundary"}, transcript, "p1", "p2"); err != nil || got {
		t.Fatalf("ordered=%v err=%v without queued_turns", got, err)
	}
	if _, err := TurnRanAfter(claudeQueuedTurns, filepath.Join(t.TempDir(), "missing.jsonl"), "p1", "p2"); err == nil {
		t.Fatal("missing transcript read as unordered")
	}
}

func TestQueuedTurnPendingReadsOnlyWhenTheCollarAsks(t *testing.T) {
	transcript := claudeTranscript(t, "0 user p1", "1 queue enqueue")
	if got, err := QueuedTurnPending(Invocation{Name: "hook-boundary"}, transcript, "p1"); err != nil || got {
		t.Fatalf("pending=%v err=%v without queued_turns", got, err)
	}
	if _, err := QueuedTurnPending(Invocation{Name: "hook-boundary", Params: map[string]string{"queued_turns": "codex"}}, transcript, "p1"); err == nil {
		t.Fatal("unknown queued_turns source accepted")
	}
	if _, err := QueuedTurnPending(claudeQueuedTurns, filepath.Join(t.TempDir(), "missing.jsonl"), "p1"); err == nil {
		t.Fatal("missing transcript read as an empty queue")
	}
	// Claude Code may be mid-write of the last record when the hook reads.
	partial := claudeTranscript(t, "0 user p1", "1 queue enqueue")
	if err := appendText(partial, `{"type":"queue-operation","operation":"deq`); err != nil {
		t.Fatal(err)
	}
	if got, err := QueuedTurnPending(claudeQueuedTurns, partial, "p1"); err != nil || !got {
		t.Fatalf("pending=%v err=%v with a partial last record", got, err)
	}
	garbled := claudeTranscript(t, "0 user p1", "1 queue enqueue")
	if err := appendText(garbled, "{\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := QueuedTurnPending(claudeQueuedTurns, garbled, "p1"); err == nil {
		t.Fatal("garbled transcript read as a queue")
	}
}

func appendText(path, text string) error {
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	if _, err := file.WriteString(text); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}
