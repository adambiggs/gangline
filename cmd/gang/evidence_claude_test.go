//go:build acceptance

package main

import (
	"bytes"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adambiggs/gangline/internal/evidence"
)

func TestEvidenceClaudeViewKeepsBodiesPrivate(t *testing.T) {
	root := t.TempDir()
	tree := strings.Repeat("a", 40)
	object := []byte("tree " + tree + "\nauthor Test <test@example.test> 1 +0000\ncommitter Test <test@example.test> 1 +0000\n\nimplementation\n")
	h := sha1.New()
	fmt.Fprintf(h, "commit %d\x00", len(object))
	h.Write(object)
	commit := hex.EncodeToString(h.Sum(nil))
	native := []byte(fmt.Sprintf(`{"type":"user","sessionId":"s","promptId":"p","timestamp":"2026-11-02T01:00:00Z","message":{"role":"user","content":"PRIVATE-PROMPT"}}
{"type":"assistant","sessionId":"s","uuid":"call-record","requestId":"request","timestamp":"2026-11-02T01:00:01Z","message":{"id":"message","role":"assistant","model":"claude-observed","stop_reason":"tool_use","content":[{"type":"tool_use","id":"call","input":{"command":"PRIVATE-COMMAND"}}]}}
{"type":"user","sessionId":"s","uuid":"result-record","timestamp":"2026-11-02T01:00:02Z","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"call","content":"PRIVATE-OUTPUT %s"}]}}
{"type":"assistant","sessionId":"s","timestamp":"2026-11-02T01:00:03Z","message":{"role":"assistant","stop_reason":"end_turn","content":[{"type":"text","text":"PRIVATE-ANSWER"}]}}
{"type":"system","subtype":"turn_duration","sessionId":"s","timestamp":"2026-11-02T01:00:04Z"}
`, commit))
	m := evidence.Manifest{Version: 1, Scope: evidence.Scope{Host: "host", Team: "team", Hitch: "hitch", Assignment: "task"}, Target: "code", Records: []evidence.Record{
		{ID: "run", Kind: "run", Provenance: "manual_import", Run: &evidence.Run{Native: "native", Harness: "claude", Session: "s", Turn: "p", Implementer: "worker"}},
		{ID: "code", Kind: "code", Provenance: "manual_import", Code: &evidence.Code{Run: "run", NativeCall: "call", Repository: "example/project", Commit: commit, Tree: tree, CommitObject: "git"}},
	}}
	for name, b := range map[string][]byte{"native": native, "git": object} {
		if err := os.WriteFile(filepath.Join(root, name), b, 0600); err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(b)
		m.Records = append(m.Records, evidence.Record{ID: name, Kind: "artifact", Provenance: "manual_import", Artifact: &evidence.Artifact{Path: name, SHA256: hex.EncodeToString(sum[:]), Size: int64(len(b)), Status: "available"}})
	}
	manifest := filepath.Join(root, "manifest.json")
	data, _ := json.Marshal(m)
	if err := os.WriteFile(manifest, data, 0600); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"verify", "show"} {
		var out, stderr bytes.Buffer
		status := run([]string{"evidence", mode, "--assignment", "task", manifest}, strings.NewReader(""), &out, &stderr)
		want := exitOK
		if mode == "verify" {
			want = exitUnknown
		}
		if status != want || strings.Contains(out.String()+stderr.String(), "PRIVATE-") {
			t.Fatal("unexpected outcome or body exposure")
		}
		var result evidence.Result
		if json.Unmarshal(out.Bytes(), &result) != nil || result.Status != "incomplete" || result.Native == nil || result.Native.Harness != "claude" || result.Native.Request != "request" || result.Coverage["session_code"] != "native_tool_result_commit" {
			t.Fatal("native identity lost or absent SDLC edges became pass")
		}
	}
}
