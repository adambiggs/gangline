package evidence

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func claudeFixtureChain() *fixture {
	f := fixtureChain()
	f.get("run").Run.Harness = "claude"
	f.artifact("native", []byte(fmt.Sprintf(`{"type":"user","sessionId":"session","promptId":"turn","uuid":"prompt-record","timestamp":"2026-11-02T01:00:00Z","message":{"role":"user","content":"PRIVATE-PROMPT"}}
{"type":"assistant","sessionId":"session","uuid":"call-record","requestId":"request","timestamp":"2026-11-02T01:00:01Z","message":{"id":"message","role":"assistant","model":"claude-observed","stop_reason":"tool_use","content":[{"type":"tool_use","id":"code-call","name":"Bash","input":{"command":"PRIVATE-COMMAND"}}]}}
{"type":"user","sessionId":"session","uuid":"result-record","timestamp":"2026-11-02T01:00:02Z","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"code-call","content":"PRIVATE-OUTPUT tested tree %s"}]}}
{"type":"assistant","sessionId":"session","uuid":"final-record","timestamp":"2026-11-02T01:00:03Z","message":{"id":"final-message","role":"assistant","model":"claude-observed","stop_reason":"end_turn","content":[{"type":"text","text":"PRIVATE-ANSWER"}]}}
{"type":"system","subtype":"turn_duration","sessionId":"session","timestamp":"2026-11-02T01:00:04Z"}
`, f.get("code").Code.Tree)))
	return f
}
func changeClaude(f *fixture, change func(string) string) {
	f.artifact("native", []byte(change(string(f.files["native.json"]))))
}
func TestClaudeEvidenceJoinsRealRecordTypes(t *testing.T) {
	f := claudeFixtureChain()
	result := f.result()
	if result.Status != "pass" || result.Native == nil || result.Native.Harness != "claude" || result.Native.Session != "session" || result.Native.Turn != "turn" || result.Native.ToolCall != "code-call" || result.Native.Message != "message" || result.Native.Request != "request" || result.Native.CallRecord != "call-record" || result.Native.ResultRecord != "result-record" || result.Native.ResultOffset == 0 {
		t.Fatalf("result %+v", result)
	}
	if result.Coverage["session_code"] != "native_tool_result_tree" || result.Coverage["observed_model"] != "claude-observed" || result.Coverage["requested_model"] != "requested-model" {
		t.Fatalf("coverage %+v", result.Coverage)
	}
	b, _ := json.Marshal(result)
	if bytes.Contains(b, []byte("PRIVATE-")) {
		t.Fatal("payload leaked into view")
	}
	changeClaude(f, func(s string) string {
		return strings.Replace(s, "PRIVATE-OUTPUT tested tree "+f.get("code").Code.Tree, f.get("code").Code.Commit, 1)
	})
	if result = f.result(); result.Status != "pass" || result.Coverage["session_code"] != "native_tool_result_commit" {
		t.Fatalf("commit witness %+v", result)
	}
}
func TestClaudeEvidenceDoesNotInventEdges(t *testing.T) {
	cases := []struct {
		name, status string
		change       func(*fixture)
	}{
		{"wrong session", "fail", func(f *fixture) { f.get("run").Run.Session = "another" }},
		{"wrong turn", "incomplete", func(f *fixture) { f.get("run").Run.Turn = "another" }},
		{"wrong call", "incomplete", func(f *fixture) { f.get("code").Code.NativeCall = "other" }},
		{"missing call", "incomplete", func(f *fixture) {
			changeClaude(f, func(s string) string { return strings.Replace(s, `"id":"code-call"`, `"id":"different"`, 1) })
		}},
		{"result for another prompt", "incomplete", func(f *fixture) {
			changeClaude(f, func(s string) string {
				return strings.Replace(s, `"uuid":"result-record"`, `"uuid":"result-record","promptId":"other"`, 1)
			})
		}},
		{"no settled boundary", "incomplete", func(f *fixture) {
			changeClaude(f, func(s string) string { return strings.Replace(s, `"turn_duration"`, `"stop_hook_summary"`, 1) })
		}},
		{"no successful stop", "incomplete", func(f *fixture) {
			changeClaude(f, func(s string) string { return strings.Replace(s, `"end_turn"`, `"max_tokens"`, 1) })
		}},
		{"partial final record", "incomplete", func(f *fixture) { changeClaude(f, func(s string) string { return strings.TrimSuffix(s, "\n") }) }},
		{"malformed record", "fail", func(f *fixture) { changeClaude(f, func(s string) string { return s + "{PRIVATE-BODY}\n" }) }},
		{"failed result", "fail", func(f *fixture) {
			changeClaude(f, func(s string) string {
				return strings.Replace(s, `"tool_use_id":"code-call"`, `"tool_use_id":"code-call","is_error":true`, 1)
			})
		}},
		{"API failure", "fail", func(f *fixture) {
			changeClaude(f, func(s string) string {
				return strings.Replace(s, `"uuid":"final-record"`, `"uuid":"final-record","isApiErrorMessage":true`, 1)
			})
		}},
		{"sidechain result", "incomplete", func(f *fixture) {
			changeClaude(f, func(s string) string {
				return strings.Replace(s, `"uuid":"result-record"`, `"uuid":"result-record","isSidechain":true`, 1)
			})
		}},
		{"compaction prompt", "incomplete", func(f *fixture) {
			changeClaude(f, func(s string) string {
				return strings.Replace(s, `"uuid":"prompt-record"`, `"uuid":"prompt-record","isCompactSummary":true`, 1)
			})
		}},
		{"other terminal session", "fail", func(f *fixture) {
			changeClaude(f, func(s string) string {
				return strings.Replace(s, `"subtype":"turn_duration","sessionId":"session"`, `"subtype":"turn_duration","sessionId":"other"`, 1)
			})
		}},
		{"missing timestamp", "fail", func(f *fixture) {
			changeClaude(f, func(s string) string { return strings.Replace(s, `"timestamp":"2026-11-02T01:00:02Z",`, "", 1) })
		}},
		{"another task CI", "fail", func(f *fixture) {
			f.artifact("jobs", []byte(strings.Replace(string(f.files["jobs.json"]), f.get("code").Code.Commit, strings.Repeat("b", 40), 1)))
		}},
		{"no tests", "incomplete", func(f *fixture) { f.get("code").Code.Tests = nil }},
		{"no review", "incomplete", func(f *fixture) { f.get("code").Code.Reviews = nil }},
		{"wrong tree result", "incomplete", func(f *fixture) {
			changeClaude(f, func(s string) string { return strings.Replace(s, f.get("code").Code.Tree, strings.Repeat("b", 40), 1) })
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := claudeFixtureChain()
			c.change(f)
			r := f.result()
			if r.Status != c.status {
				t.Fatalf("want %s got %+v", c.status, r)
			}
			b, _ := json.Marshal(r)
			if bytes.Contains(b, []byte("PRIVATE-")) {
				t.Fatal("payload leaked")
			}
		})
	}
}
func TestClaudeWitnessTextAndDuplicateIdentity(t *testing.T) {
	f := claudeFixtureChain()
	changeClaude(f, func(s string) string {
		return strings.Replace(s, `"content":"PRIVATE-OUTPUT tested tree `+f.get("code").Code.Tree+`"`, `"content":[{"type":"text","text":"`+f.get("code").Code.Tree+`"},{"type":"image","text":"PRIVATE-IGNORED"}]`, 1)
	})
	if r := f.result(); r.Status != "pass" {
		t.Fatalf("text blocks %+v", r)
	}
	original := string(f.files["native.json"])
	lines := strings.SplitAfter(original, "\n")
	f.artifact("native", []byte(original+lines[2]))
	if r := f.result(); r.Status != "pass" {
		t.Fatalf("identical duplicate %+v", r)
	}
	f.artifact("native", []byte(original+strings.Replace(lines[2], f.get("code").Code.Tree, "PRIVATE-CONFLICT", 1)))
	if r := f.result(); r.Status != "fail" {
		t.Fatalf("conflicting UUID %+v", r)
	}
}
func TestClaudeCollectedEvidenceRestoresWithoutOriginal(t *testing.T) {
	f := claudeFixtureChain()
	copied := map[string][]byte{}
	m, _, err := Collect(f.m, "assignment", memoryReader(f.files), func(p string, b []byte) error { copied[p] = bytes.Clone(b); return nil })
	if err != nil {
		t.Fatal(err)
	}
	f.files = nil
	if r := Verify(m, "assignment", "ci", memoryReader(copied)); r.Status != "pass" || r.Native.Harness != "claude" {
		t.Fatalf("restored %+v", r)
	}
}
