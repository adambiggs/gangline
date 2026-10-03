package harness

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var claudeSessionMoves = Invocation{Name: "hook-boundary", Params: map[string]string{"session_moves": "claude-transcript"}}

// writeTranscript writes session's transcript into dir, one record per line,
// and returns its path.
func writeTranscript(t *testing.T, dir, session string, lines ...string) string {
	t.Helper()
	path := filepath.Join(dir, session+".jsonl")
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// The records are shaped as Claude Code writes them when a conversation moves
// to a background session.
const (
	movePrompt  = `{"type":"user","sessionId":"old","promptId":"p1","timestamp":"2026-10-03T07:15:10Z","message":{"role":"user","content":"hi"}}`
	moveReply   = `{"type":"assistant","sessionId":"old","timestamp":"2026-10-03T07:15:12Z","message":{"role":"assistant","stop_reason":"end_turn"}}`
	moveCost    = `{"type":"cost-state","sessionId":"old","totalCostUSD":0.04}`
	moveRecord  = `{"type":"continued-in","timestamp":"2026-10-03T07:15:48.447Z","sessionId":"old","continuedInSessionId":"new"}`
	moveAnother = `{"type":"continued-in","timestamp":"2026-10-03T07:15:48.447Z","sessionId":"old","continuedInSessionId":"other"}`
)

func TestSessionMovedFollowsContinuedInRecord(t *testing.T) {
	dir := t.TempDir()
	old := writeTranscript(t, dir, "old", movePrompt, moveReply, moveCost, moveRecord)
	if moved, err := SessionMoved(claudeSessionMoves, old, "old", "new", filepath.Join(dir, "new.jsonl")); err != nil || !moved {
		t.Fatalf("moved=%v err=%v for a recorded move", moved, err)
	}
	if moved, err := SessionMoved(Invocation{Name: "hook-boundary"}, old, "old", "new", filepath.Join(dir, "new.jsonl")); err != nil || moved {
		t.Fatalf("moved=%v err=%v without session_moves", moved, err)
	}
	if _, err := SessionMoved(Invocation{Name: "hook-boundary", Params: map[string]string{"session_moves": "codex"}}, old, "old", "new", filepath.Join(dir, "new.jsonl")); err == nil {
		t.Fatal("unknown session_moves source accepted")
	}
}

func TestSessionMovedRefusesUnprovenSession(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		name  string
		lines []string
		next  string
		path  string
	}{
		{"no record", []string{movePrompt, moveReply}, "new", ""},
		{"record names another session", []string{movePrompt, moveAnother}, "new", ""},
		{"record of another source session", []string{movePrompt, strings.Replace(moveRecord, `"sessionId":"old"`, `"sessionId":"third"`, 1)}, "new", ""},
		{"prompt after the record", []string{moveRecord, movePrompt}, "new", ""},
		{"reply after the record", []string{moveRecord, moveReply}, "new", ""},
		{"later record names another session", []string{moveRecord, moveAnother}, "new", ""},
		{"transcript in another directory", []string{moveRecord}, "new", filepath.Join(t.TempDir(), "new.jsonl")},
		{"transcript under another name", []string{moveRecord}, "new", filepath.Join(dir, "other.jsonl")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			old := writeTranscript(t, dir, "old", tc.lines...)
			path := tc.path
			if path == "" {
				path = filepath.Join(dir, tc.next+".jsonl")
			}
			if moved, err := SessionMoved(claudeSessionMoves, old, "old", tc.next, path); err != nil || moved {
				t.Fatalf("moved=%v err=%v", moved, err)
			}
		})
	}
}

func TestSessionMovedFollowsSuccessiveMoves(t *testing.T) {
	dir := t.TempDir()
	old := writeTranscript(t, dir, "old", movePrompt, strings.Replace(moveRecord, `"new"`, `"mid"`, 1))
	writeTranscript(t, dir, "mid", strings.Replace(movePrompt, `"old"`, `"mid"`, 1), `{"type":"continued-in","sessionId":"mid","continuedInSessionId":"new"}`)
	if moved, err := SessionMoved(claudeSessionMoves, old, "old", "new", filepath.Join(dir, "new.jsonl")); err != nil || !moved {
		t.Fatalf("moved=%v err=%v across two recorded moves", moved, err)
	}
	// Records that lead back to an earlier session never reach another one.
	writeTranscript(t, dir, "mid", `{"type":"continued-in","sessionId":"mid","continuedInSessionId":"old"}`)
	if moved, err := SessionMoved(claudeSessionMoves, old, "old", "new", filepath.Join(dir, "new.jsonl")); err != nil || moved {
		t.Fatalf("moved=%v err=%v through a cycle of records", moved, err)
	}
}

func TestSessionMovedIgnoresUnfinishedLine(t *testing.T) {
	dir := t.TempDir()
	old := filepath.Join(dir, "old.jsonl")
	if err := os.WriteFile(old, []byte(moveRecord+"\n"+`{"type":"user","sessionId":"ol`), 0o600); err != nil {
		t.Fatal(err)
	}
	if moved, err := SessionMoved(claudeSessionMoves, old, "old", "new", filepath.Join(dir, "new.jsonl")); err != nil || !moved {
		t.Fatalf("moved=%v err=%v with a line still being written", moved, err)
	}
}
