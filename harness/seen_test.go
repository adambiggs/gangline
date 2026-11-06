package harness

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func writeTranscript(t *testing.T, lines ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "transcript.jsonl")
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func envelopeFor(token string) string {
	return `[gang:lead#` + token + `] note [/gang:lead#` + token + `]\n[gang ctx=1k/1M at=11-06T10:00:00Z /]`
}

func TestSeenTokensCountsOnlyInputTheModelReceived(t *testing.T) {
	const (
		turn     = "1111111111111111"
		midTurn  = "2222222222222222"
		enqueued = "3333333333333333"
		tool     = "4444444444444444"
		recalled = "5555555555555555"
		subagent = "6666666666666666"
		hook     = "7777777777777777"
		image    = "8888888888888888"
		quoted   = "9999999999999999"
	)
	all := []string{turn, midTurn, enqueued, tool, recalled, subagent, hook, image, quoted}
	for _, tc := range []struct {
		name  string
		seen  Invocation
		lines []string
		want  []string
		since string
	}{
		{
			name: "claude",
			seen: Invocation{Name: "claude-transcript"},
			lines: []string{
				`{"type":"file-history-snapshot","messageId":"m"}`,
				`{"type":"user","timestamp":"2026-11-06T10:00:00Z","message":{"role":"user","content":"` + envelopeFor(turn) + `"}}`,
				`{"type":"queue-operation","operation":"enqueue","timestamp":"2026-11-06T10:00:01Z","content":"` + envelopeFor(enqueued) + `"}`,
				`{"type":"queue-operation","operation":"enqueue","timestamp":"2026-11-06T10:00:01Z","content":"` + envelopeFor(midTurn) + `"}`,
				`{"type":"attachment","timestamp":"2026-11-06T10:00:02Z","attachment":{"type":"queued_command","prompt":"` + envelopeFor(midTurn) + `"}}`,
				`{"type":"user","timestamp":"2026-11-06T10:00:03Z","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"t","content":"` + envelopeFor(tool) + `"}]}}`,
				`{"type":"last-prompt","lastPrompt":"` + envelopeFor(recalled) + `"}`,
				`{"type":"user","isSidechain":true,"timestamp":"2026-11-06T10:00:04Z","message":{"role":"user","content":"` + envelopeFor(subagent) + `"}}`,
				`{"type":"user","isMeta":true,"timestamp":"2026-11-06T10:00:05Z","message":{"role":"user","content":"` + envelopeFor(hook) + `"}}`,
				`{"type":"user","timestamp":"2026-11-06T10:00:06Z","message":{"role":"user","content":[{"type":"image","source":{}},{"type":"text","text":"` + envelopeFor(image) + `"}]}}`,
				`{"type":"user","timestamp":"2026-11-06T10:00:07Z","message":{"role":"user","content":"a body naming ` + quoted + ` is not an opener"}}`,
			},
			want:  []string{turn, midTurn, image},
			since: "2026-11-06T10:00:00Z",
		},
		{
			name: "codex",
			seen: Invocation{Name: "codex-session-log"},
			lines: []string{
				`{"timestamp":"2026-11-06T09:59:00Z","type":"session_meta","payload":{"id":"s"}}`,
				`{"timestamp":"2026-11-06T10:00:00Z","type":"event_msg","payload":{"type":"task_started"}}`,
				`{"timestamp":"2026-11-06T10:00:00Z","type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"` + envelopeFor(turn) + `"}]}}`,
				`{"timestamp":"2026-11-06T10:00:01Z","type":"response_item","payload":{"type":"custom_tool_call_output","output":"` + envelopeFor(tool) + `"}}`,
				`{"timestamp":"2026-11-06T10:00:01Z","type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"` + envelopeFor(midTurn) + `"}]}}`,
				`{"timestamp":"2026-11-06T10:00:02Z","type":"compacted","payload":{"message":"` + envelopeFor(recalled) + `"}}`,
				`{"timestamp":"2026-11-06T10:00:03Z","type":"response_item","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"` + envelopeFor(hook) + `"}]}}`,
			},
			want:  []string{turn, midTurn},
			since: "2026-11-06T09:59:00Z",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			seen, since, err := SeenTokens(tc.seen, writeTranscript(t, tc.lines...), all)
			if err != nil {
				t.Fatal(err)
			}
			if want, _ := time.Parse(time.RFC3339, tc.since); !since.Equal(want) {
				t.Fatalf("since = %v, want %v", since, want)
			}
			for _, token := range all {
				if got, want := seen[token], slices.Contains(tc.want, token); got != want {
					t.Errorf("token %s seen = %v, want %v", token, got, want)
				}
			}
		})
	}
}

// A record still being written is not evidence, and a transcript larger
// than the window reports the time of the first record it read.
func TestSeenTokensReadsCompleteRecordsInTheWindow(t *testing.T) {
	const token = "abcdefabcdefabcd"
	record := `{"type":"user","timestamp":"2026-11-06T10:00:00Z","message":{"role":"user","content":"` + envelopeFor(token) + `"}}`
	path := writeTranscript(t, record)
	partial := strings.Replace(record, "10:00:00", "10:00:09", 1)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	padding := `{"type":"progress","timestamp":"2026-11-06T11:00:00Z","data":"` + strings.Repeat("x", transcriptWindow) + `"}` + "\n"
	later := `{"type":"progress","timestamp":"2026-11-06T12:00:00Z"}` + "\n"
	if _, err := f.WriteString(padding + later + strings.Replace(partial, "abcdefabcdefabcd", "0000000000000000", 2)); err != nil {
		t.Fatal(err)
	}
	f.Close()
	seen, since, err := SeenTokens(Invocation{Name: "claude-transcript"}, path, []string{token, "0000000000000000"})
	if err != nil {
		t.Fatal(err)
	}
	if len(seen) != 0 {
		t.Fatalf("seen = %v outside the window or in a partial record", seen)
	}
	if want := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC); !since.Equal(want) {
		t.Fatalf("since = %v, want %v", since, want)
	}
}

func TestSeenTokensRefusesUnknownReaders(t *testing.T) {
	path := writeTranscript(t, `{"type":"user"}`)
	if _, _, err := SeenTokens(Invocation{Name: "screen"}, path, []string{"1111111111111111"}); err == nil {
		t.Fatal("unknown reader accepted")
	}
	if _, _, err := SeenTokens(Invocation{Name: "claude-transcript"}, path+".missing", []string{"1111111111111111"}); err == nil {
		t.Fatal("missing transcript accepted")
	}
	broken := writeTranscript(t, `{"type":"user"`)
	if _, _, err := SeenTokens(Invocation{Name: "claude-transcript"}, broken, []string{"1111111111111111"}); err == nil {
		t.Fatal("malformed record accepted")
	}
}
