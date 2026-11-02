package harness

import (
	"bytes"
	"strings"
	"testing"
)

const selectedClaude = `{"type":"user","sessionId":"s","promptId":"p","uuid":"p-record","timestamp":"2026-11-02T01:00:00Z","message":{"role":"user","content":"synthetic task"}}
{"type":"assistant","sessionId":"s","uuid":"a","timestamp":"2026-11-02T01:00:01Z","message":{"role":"assistant","model":"observed","stop_reason":"tool_use","content":[{"type":"tool_use","id":"call"}]}}
{"type":"user","sessionId":"s","uuid":"r","timestamp":"2026-11-02T01:00:02Z","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"call","content":"witness"}]}}
{"type":"assistant","sessionId":"s","uuid":"f","timestamp":"2026-11-02T01:00:03Z","message":{"role":"assistant","stop_reason":"end_turn","content":[]}}
{"type":"system","subtype":"turn_duration","sessionId":"s","timestamp":"2026-11-02T01:00:04Z"}
`

func TestClaudeSelectedTurnBoundaries(t *testing.T) {
	lines := strings.SplitAfter(selectedClaude, "\n")
	for _, tc := range []struct {
		name, input       string
		complete, witness bool
	}{
		{"selected", selectedClaude, true, true},
		{"timestamp order", lines[0] + lines[1] + lines[4] + lines[2] + lines[3], true, true},
		{"EOF alone", lines[0] + lines[1] + lines[2], false, true},
		{"end_turn alone", lines[0] + lines[1] + lines[2] + lines[3], false, true},
		{"duration without end_turn", lines[0] + lines[1] + lines[2] + lines[4], false, true},
		{"result before call", lines[0] + strings.Replace(lines[2], "01:00:02Z", "01:00:00Z", 1) + lines[1] + lines[3] + lines[4], true, false},
		{"different prompt before final", lines[0] + lines[1] + lines[2] + strings.ReplaceAll(strings.ReplaceAll(strings.Replace(lines[0], "01:00:00Z", "01:00:02Z", 1), `"p"`, `"other"`), "p-record", "other-record") + lines[3] + lines[4], false, true},
		{"selected prompt resumed", selectedClaude + strings.ReplaceAll(strings.Replace(lines[0], "01:00:00Z", "01:00:05Z", 1), "p-record", "resumed"), false, false},
		{"unknown prompt shape", strings.Replace(selectedClaude, `"content":"synthetic task"`, `"content":{"new":"shape"}`, 1), false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ReadClaudeTurn(strings.NewReader(tc.input), "s", "p", "call")
			if err != nil || got.Complete != tc.complete || got.Witness != tc.witness {
				t.Fatalf("got %+v err %v", got, err)
			}
		})
	}
}
func TestClaudeReaderPartialAndBoundedHistory(t *testing.T) {
	partial := selectedClaude + `{"unfinished"`
	got, err := ReadClaudeTurn(strings.NewReader(partial), "s", "p", "call")
	if err != nil || got.Offset != int64(len(selectedClaude)) || !got.Complete {
		t.Fatalf("partial %+v %v", got, err)
	}
	firstEnd := strings.IndexByte(selectedClaude, '\n') + 1
	huge := selectedClaude[:firstEnd] + `{"type":"unknown","data":"` + strings.Repeat("x", transcriptWindow) + `"}` + "\n" + selectedClaude[firstEnd:]
	got, err = ReadClaudeTurn(strings.NewReader(huge), "s", "p", "call")
	if err != nil || got.Complete || got.Witness {
		t.Fatalf("evicted prompt guessed: %+v %v", got, err)
	}
	original := []byte(selectedClaude)
	_, err = ReadClaudeTurn(bytes.NewReader(original), "other", "p", "call")
	if err == nil || string(original) != selectedClaude {
		t.Fatal("identity accepted or input changed")
	}
}
