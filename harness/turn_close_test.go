//go:build acceptance

package harness

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

var claudeTurnClose = Invocation{Name: "hook-boundary", Params: map[string]string{"turn_close": "claude-transcript"}}

func TestTurnClosed(t *testing.T) {
	start := time.Date(2026, 10, 2, 10, 55, 0, 0, time.UTC)
	for _, test := range []struct {
		name        string
		records     []string
		closed      bool
		at          float64
		interrupted bool
	}{
		{"a normal turn ends", []string{"0 user p1", "1 assistant -", "2 system stop_hook_summary", "2.5 system turn_duration"}, true, 2.5, false},
		{"a running turn", []string{"0 user p1", "1 assistant -"}, false, 0, false},
		{"only the turn before it ended", []string{"0 user p0", "1 system turn_duration", "2 user p1", "3 assistant -"}, false, 0, false},
		{"the turn is not in the transcript", []string{"0 user p0", "1 system turn_duration"}, false, 0, false},
		{"Escape writes its note before the end", []string{"0 user p1", "1 assistant -", "2 user p1 [Request interrupted by user]", "2.1 system turn_duration"}, true, 2.1, true},
		// A prompt submitted at a tool boundary joins the running turn.
		{"a prompt folded into the turn", []string{"0 user p1", "1 assistant -", "10 queue enqueue", "14 user p1", "14.1 queue remove", "16 system stop_hook_summary", "16.1 system turn_duration"}, true, 16.1, false},
		{"a prompt still queued at the end", []string{"0 user p1", "1 queue enqueue", "2 assistant -", "3 system turn_duration"}, false, 0, false},
		// Claude Code writes the dequeue ahead of the turn's last records.
		{"the next turn's dequeue written first", []string{"0 user p1", "1 queue enqueue", "4 queue dequeue", "2 assistant -", "3 system turn_duration"}, false, 0, false},
		{"the queued turn runs", []string{"0 user p1", "1 queue enqueue", "4 queue dequeue", "3 system turn_duration", "5 user p2", "6 assistant -"}, false, 0, false},
		{"the queued turn ends", []string{"0 user p1", "1 queue enqueue", "4 queue dequeue", "3 system turn_duration", "5 user p2", "6 assistant -", "7 system turn_duration"}, true, 7, false},
		{"an interrupted turn's note does not mark the next", []string{"0 user p1", "1 user p1 [Request interrupted by user]", "1.1 system turn_duration", "2 user p2", "3 system turn_duration"}, true, 3, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			end, closed, err := TurnClosed(claudeTurnClose, claudeTranscript(t, test.records...), "p1")
			if err != nil {
				t.Fatal(err)
			}
			want := start.Add(time.Duration(test.at * float64(time.Second)))
			if closed != test.closed || closed && (!end.At.Equal(want) || end.Interrupted != test.interrupted) {
				t.Fatalf("closed=%v end=%+v, want closed=%v at %v interrupted=%v", closed, end, test.closed, want, test.interrupted)
			}
		})
	}
}

// Escape on a permission dialog fires no hook. Claude Code records the
// rejected tool result, a text-block note and the turn's end.
func TestTurnClosedOnEscapedPermissionDialog(t *testing.T) {
	path := filepath.Join(t.TempDir(), "transcript.jsonl")
	lines := `{"type":"user","promptId":"db34","timestamp":"2026-11-06T15:13:17.331Z","message":{"role":"user","content":"Use the Bash tool to run curl"}}
{"type":"assistant","timestamp":"2026-11-06T15:13:19.000Z","message":{"role":"assistant","content":[]}}
{"type":"user","promptId":"db34","timestamp":"2026-11-06T15:13:38.155Z","message":{"role":"user","content":[{"type":"tool_result","content":"The user doesn't want to proceed with this tool use.","is_error":true,"tool_use_id":"toolu_1"}]}}
{"type":"user","promptId":"db34","timestamp":"2026-11-06T15:13:38.156Z","message":{"role":"user","content":[{"type":"text","text":"[Request interrupted by user for tool use]"}]}}
{"type":"system","subtype":"turn_duration","durationMs":1209,"isMeta":false,"timestamp":"2026-11-06T15:13:38.157Z"}
`
	if err := os.WriteFile(path, []byte(lines), 0o600); err != nil {
		t.Fatal(err)
	}
	end, closed, err := TurnClosed(claudeTurnClose, path, "db34")
	if err != nil || !closed || !end.Interrupted || !end.At.Equal(time.Date(2026, 11, 6, 15, 13, 38, 157e6, time.UTC)) {
		t.Fatalf("closed=%v end=%+v err=%v", closed, end, err)
	}
}

// A collar that reads no transcript for turn ends closes nothing.
func TestTurnClosedNeedsTheCollarSource(t *testing.T) {
	path := claudeTranscript(t, "0 user p1", "1 system turn_duration")
	if _, closed, err := TurnClosed(Invocation{Name: "hook-boundary"}, path, "p1"); err != nil || closed {
		t.Fatalf("closed=%v err=%v", closed, err)
	}
	if _, _, err := TurnClosed(Invocation{Name: "hook-boundary", Params: map[string]string{"turn_close": "screen"}}, path, "p1"); err == nil {
		t.Fatal("unknown turn_close source accepted")
	}
}
