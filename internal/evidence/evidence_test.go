package evidence

import (
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

type fixture struct {
	m     Manifest
	files map[string][]byte
}

func (f *fixture) add(id, kind string, value any) {
	r := Record{ID: id, Kind: kind, Provenance: "producer_export"}
	switch v := value.(type) {
	case *Artifact:
		r.Artifact = v
	case *Run:
		r.Run = v
	case *Test:
		r.Test = v
	case *Code:
		r.Code = v
	case *Review:
		r.Review = v
	case *CI:
		r.CI = v
	case *Usage:
		r.Usage = v
	case *Deployment:
		r.Deployment = v
	}
	f.m.Records = append(f.m.Records, r)
}
func (f *fixture) artifact(id string, data []byte) {
	sum := sha256.Sum256(data)
	a := &Artifact{Path: id + ".json", SHA256: hex.EncodeToString(sum[:]), Size: int64(len(data)), Status: "available"}
	f.files[a.Path] = data
	for i, r := range f.m.Records {
		if r.ID == id {
			f.m.Records[i].Artifact = a
			return
		}
	}
	f.add(id, "artifact", a)
}
func (f *fixture) json(id string, v any) { b, _ := json.Marshal(v); f.artifact(id, b) }
func (f *fixture) get(id string) *Record {
	for i := range f.m.Records {
		if f.m.Records[i].ID == id {
			return &f.m.Records[i]
		}
	}
	panic(id)
}
func (f *fixture) result() Result {
	return Verify(f.m, "assignment", "ci", func(path string, _ int64) ([]byte, error) {
		b, ok := f.files[path]
		if !ok {
			return nil, fmt.Errorf("missing")
		}
		return b, nil
	})
}
func fixtureChain() *fixture {
	f := &fixture{m: Manifest{Version: 1, Scope: Scope{"host", "team", "hitch", "assignment"}, Target: "code"}, files: map[string][]byte{}}
	tree := strings.Repeat("a", 40)
	object := []byte("tree " + tree + "\nauthor Test <test@example.test> 1 +0000\ncommitter Test <test@example.test> 1 +0000\n\nimplementation\n")
	h := sha1.New()
	fmt.Fprintf(h, "commit %d\x00", len(object))
	h.Write(object)
	commit := hex.EncodeToString(h.Sum(nil))
	f.artifact("git", object)
	f.artifact("native", []byte("{\"type\":\"session_meta\",\"payload\":{\"id\":\"session\"}}\n"+"{\"timestamp\":\"2026-11-02T01:00:00Z\",\"type\":\"turn_context\",\"payload\":{\"model\":\"observed-model\"}}\n"+"{\"timestamp\":\"2026-11-02T01:00:01Z\",\"type\":\"event_msg\",\"payload\":{\"type\":\"task_complete\",\"turn_id\":\"turn\"}}\n"))
	native := string(f.files["native.json"])
	start := `{"timestamp":"2026-11-02T00:59:59Z","type":"event_msg","payload":{"type":"task_started","turn_id":"turn"}}` + "\n"
	output, _ := json.Marshal(map[string]any{"type": "response_item", "payload": map[string]any{"type": "function_call_output", "call_id": "code-call", "output": "tested tree " + tree}})
	split := strings.Index(native, "\n") + 1
	f.artifact("native", []byte(native[:split]+start+string(output)+"\n"+native[split:]))
	f.add("run", "run", &Run{Native: "native", Harness: "codex", Session: "session", Turn: "turn", Implementer: "implementer", RequestedModel: "requested-model", RequestedEffort: "high"})
	f.add("code", "code", &Code{Run: "run", Repository: "example/project", Commit: commit, CommitObject: "git", NativeCall: "code-call", Tree: tree, Tests: []string{"test"}, Reviews: []string{"review"}, CI: []string{"ci"}})
	f.add("test", "test", &Test{Run: "run", Result: "test-result"})
	zero := 0
	f.json("test-result", TestResult{Version: 1, ID: "test", Run: "run", InputTree: tree, Argv: []string{"test/gate.sh"}, CWD: "/project", ExitCode: &zero, Outcome: "exited", Output: "test-output"})
	f.artifact("test-output", []byte("test subprocess output\n"))
	f.add("review", "review", &Review{Result: "review-result"})
	f.json("review-result", ReviewResult{Version: 1, ID: "review", Repository: "example/project", Commit: commit, Reviewer: "independent", Verdict: "approved", Unresolved: []string{}, Report: "review-output"})
	f.artifact("review-output", []byte("review scope and findings\n"))
	f.add("ci", "ci", &CI{Jobs: "jobs", RunID: 123, Attempt: 2, RequiredJobs: []string{"check"}})
	f.artifact("jobs", []byte(fmt.Sprintf(`{"jobs":[{"id":456,"run_id":123,"run_attempt":2,"head_sha":%q,"name":"check","status":"completed","conclusion":"success","html_url":"https://github.com/example/project/actions/runs/123/job/456"}]}`, commit)))
	return f
}
func changeJSON(f *fixture, id string, change func(map[string]any)) {
	var value map[string]any
	_ = json.Unmarshal(f.files[id+".json"], &value)
	change(value)
	f.json(id, value)
}
func TestVerifyActualEdges(t *testing.T) {
	f := fixtureChain()
	got := f.result()
	if got.Status != "pass" || len(got.Issues) != 0 {
		t.Fatalf("%+v", got)
	}
	if got.Coverage["deployment"] != "not_exercised" || got.Coverage["cost"] != "unavailable" || got.Coverage["requested_model"] == got.Coverage["observed_model"] {
		t.Fatalf("coverage: %v", got.Coverage)
	}
	if len(got.Sources) != 7 {
		t.Fatalf("sources: %+v", got.Sources)
	}
}
func TestVerifyRejectsBrokenEdges(t *testing.T) {
	cases := []struct {
		name, status string
		mutate       func(*fixture)
	}{
		{"wrong artifact bytes", "fail", func(f *fixture) { f.files["test-output.json"] = []byte("fake success") }},
		{"missing artifact", "incomplete", func(f *fixture) { delete(f.files, "native.json") }},
		{"wrong session", "fail", func(f *fixture) { f.get("run").Run.Session = "another" }},
		{"wrong turn", "incomplete", func(f *fixture) { f.get("run").Run.Turn = "another" }},
		{"partial tail", "incomplete", func(f *fixture) { f.artifact("native", append(f.files["native.json"], []byte("{\"partial\"")...)) }},
		{"failed native", "fail", func(f *fixture) {
			f.artifact("native", []byte(strings.Replace(string(f.files["native.json"]), `"type":"task_complete","turn_id":"turn"`, `"type":"task_complete","turn_id":"turn","error":{"message":"failed"}`, 1)))
		}},
		{"wrong tree", "fail", func(f *fixture) { f.get("code").Code.Tree = strings.Repeat("b", 40) }},
		{"wrong commit", "fail", func(f *fixture) { f.get("code").Code.Commit = strings.Repeat("b", 40) }},
		{"no tests", "incomplete", func(f *fixture) { f.get("code").Code.Tests = nil }},
		{"wrong test tree", "fail", func(f *fixture) {
			changeJSON(f, "test-result", func(m map[string]any) { m["input_tree"] = strings.Repeat("b", 40) })
		}},
		{"worker status only", "fail", func(f *fixture) { f.artifact("test-result", []byte(`{"worker_exit":0,"text":"tests passed"}`)) }},
		{"missing test exit", "incomplete", func(f *fixture) { changeJSON(f, "test-result", func(m map[string]any) { delete(m, "exit_code") }) }},
		{"test timeout", "fail", func(f *fixture) { changeJSON(f, "test-result", func(m map[string]any) { m["outcome"] = "timeout" }) }},
		{"test failure", "fail", func(f *fixture) { changeJSON(f, "test-result", func(m map[string]any) { m["exit_code"] = 1 }) }},
		{"test other run", "fail", func(f *fixture) { changeJSON(f, "test-result", func(m map[string]any) { m["run"] = "other" }) }},
		{"self review", "fail", func(f *fixture) {
			changeJSON(f, "review-result", func(m map[string]any) { m["reviewer"] = "implementer" })
		}},
		{"stale review", "fail", func(f *fixture) {
			changeJSON(f, "review-result", func(m map[string]any) { m["commit"] = strings.Repeat("b", 40) })
		}},
		{"unresolved review", "fail", func(f *fixture) {
			changeJSON(f, "review-result", func(m map[string]any) { m["unresolved"] = []string{"F1"} })
		}},
		{"missing review", "incomplete", func(f *fixture) { f.get("code").Code.Reviews = nil }},
		{"stale CI", "fail", func(f *fixture) {
			f.artifact("jobs", []byte(strings.Replace(string(f.files["jobs.json"]), f.get("code").Code.Commit, strings.Repeat("b", 40), 1)))
		}},
		{"CI other attempt", "fail", func(f *fixture) { f.get("ci").CI.Attempt = 3 }},
		{"CI skipped", "fail", func(f *fixture) {
			f.artifact("jobs", []byte(strings.Replace(string(f.files["jobs.json"]), `"success"`, `"skipped"`, 1)))
		}},
		{"CI missing check", "incomplete", func(f *fixture) { f.get("ci").CI.RequiredJobs = []string{"other"} }},
		{"CI queued", "incomplete", func(f *fixture) {
			f.artifact("jobs", []byte(strings.Replace(string(f.files["jobs.json"]), `"completed"`, `"queued"`, 1)))
		}},
		{"CI other repository", "fail", func(f *fixture) {
			f.artifact("jobs", []byte(strings.Replace(string(f.files["jobs.json"]), "example/project", "other/project", 1)))
		}},
		{"missing CI", "incomplete", func(f *fixture) { f.get("code").Code.CI = nil }},
	}
	reasons := map[string]string{"wrong artifact bytes": "artifact bytes do not match digest and size", "missing artifact": "artifact unreadable", "wrong session": "native transcript identity or records invalid", "wrong turn": "native successful terminal boundary missing", "partial tail": "native transcript has a partial final record", "failed native": "native turn ended with failure", "wrong tree": "Git commit object does not match commit and tested tree", "wrong commit": "Git commit object does not match commit and tested tree", "no tests": "tested tree evidence missing", "wrong test tree": "test identity or input tree mismatch", "worker status only": "invalid structured test result", "missing test exit": "test exit status unavailable", "test timeout": "test process did not succeed", "test failure": "test process did not succeed", "test other run": "test identity or input tree mismatch", "self review": "independent reviewer identity missing", "stale review": "review targets different code", "unresolved review": "review has no approval or has unresolved findings", "missing review": "independent review missing", "stale CI": "CI job source identity mismatch", "CI other attempt": "CI job source identity mismatch", "CI skipped": "required CI job did not succeed", "CI missing check": "required CI job absent", "CI queued": "required CI job not completed", "CI other repository": "CI job source identity mismatch", "missing CI": "hosted CI evidence missing"}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			f := fixtureChain()
			tt.mutate(f)
			got := f.result()
			if got.Status != tt.status || !hasReason(got, reasons[tt.name]) {
				t.Fatalf("want %s, got %+v", tt.status, got)
			}
		})
	}
}
func TestWithheldPayloadNeverOpenedOrPrinted(t *testing.T) {
	f := fixtureChain()
	f.get("native").Artifact = &Artifact{Status: "withheld", Path: "SENTINEL-SECRET"}
	got := Verify(f.m, "assignment", "ci", func(path string, _ int64) ([]byte, error) {
		if strings.Contains(path, "SENTINEL") {
			t.Fatal("opened withheld path")
		}
		return f.files[path], nil
	})
	b, _ := json.Marshal(got)
	if strings.Contains(string(b), "SENTINEL") || got.Status != "incomplete" {
		t.Fatalf("unsafe result %s", b)
	}
	for _, input := range []string{`{"SENTINEL-SECRET":true}`, `{"version":"SENTINEL-SECRET"}`, `{"version":1} {"SENTINEL-SECRET":true}`} {
		_, err := Decode([]byte(input))
		if err == nil || strings.Contains(err.Error(), "SENTINEL") {
			t.Fatalf("unsafe error %v", err)
		}
	}
}
func TestDecodeVersionIdentityAndDuplicateConflicts(t *testing.T) {
	f := fixtureChain()
	f.m.Records = append(f.m.Records, f.m.Records[0])
	b, _ := json.Marshal(f.m)
	m, err := Decode(b)
	if err != nil || len(m.Records) != len(f.m.Records)-1 {
		t.Fatalf("duplicate import: %v", err)
	}
	f.m.Records[len(f.m.Records)-1].Provenance = "manual_import"
	b, _ = json.Marshal(f.m)
	if _, err := Decode(b); err == nil {
		t.Fatal("conflicting identity accepted")
	}
	f = fixtureChain()
	f.m.Version = 2
	b, _ = json.Marshal(f.m)
	if _, err := Decode(b); err == nil {
		t.Fatal("future version accepted")
	}
}
func TestAssignmentAndStageCannotOverclaim(t *testing.T) {
	f := fixtureChain()
	read := func(string, int64) ([]byte, error) { t.Fatal("unexpected read"); return nil, nil }
	if got := Verify(f.m, "other", "ci", read); got.Status != "fail" {
		t.Fatal(got)
	}
	if got := Verify(f.m, "assignment", "deployed", read); got.Status != "not_exercised" {
		t.Fatal(got)
	}
}

func TestNativeCodeWitnessCannotUseAnotherTurnOrTree(t *testing.T) {
	for _, change := range []func(string) string{
		func(s string) string {
			return strings.Replace(s, `"output":"tested tree `+strings.Repeat("a", 40)+`"`, `"output":"tested tree `+strings.Repeat("b", 40)+`"`, 1)
		},
		func(s string) string { return strings.Replace(s, `"turn_id":"turn"`, `"turn_id":"other"`, 1) },
		func(s string) string {
			return strings.Replace(s, `"call_id":"code-call"`, `"call_id":"another-call"`, 1)
		},
	} {
		f := fixtureChain()
		f.artifact("native", []byte(change(string(f.files["native.json"]))))
		got := f.result()
		if got.Status != "incomplete" || !hasReason(got, "native tool result does not witness code identity in selected turn") {
			t.Fatalf("unbound code accepted: %+v", got)
		}
	}
}
func TestShowRequiredChecksAndProvenance(t *testing.T) {
	f := fixtureChain()
	f.get("native").Provenance = "manual_import"
	f.get("native").Artifact.Checkpoint = "unverified-checkpoint"
	changeJSON(f, "test-result", func(m map[string]any) { m["argv"] = []string{"true"} })
	got := f.result()
	if got.Status != "pass" || got.Coverage["required_checks"] != "caller_declared" {
		t.Fatalf("%+v", got)
	}
	if len(got.Checks) != 2 || got.Checks[0].Argv[0] != "true" || got.Checks[1].Job != "check" || got.Checks[1].RunID != 123 {
		t.Fatalf("checks missing: %+v", got.Checks)
	}
	for _, s := range got.Sources {
		if s.Record == "native" && (s.Provenance != "manual_import" || s.CheckpointVerified) {
			t.Fatal(s)
		}
	}
	f.m.Required = &Requirements{Tests: [][]string{{"test/gate.sh"}}, Jobs: []string{"required-ci"}}
	got = f.result()
	if got.Status != "incomplete" || !hasReason(got, "required test command absent") || !hasReason(got, "required policy CI job absent") {
		t.Fatalf("policy ignored: %+v", got)
	}
	f.m.Required = &Requirements{Tests: [][]string{{"true"}}, Jobs: []string{"check"}}
	if got = f.result(); got.Status != "pass" || got.Coverage["required_checks"] != "explicit_manifest_policy" {
		t.Fatal(got)
	}
}
func TestReviewerIdentityNormalization(t *testing.T) {
	for _, name := range []string{"Implementer", "implementer ", "IMPLEMENTER", "  "} {
		f := fixtureChain()
		changeJSON(f, "review-result", func(m map[string]any) { m["reviewer"] = name })
		got := f.result()
		if got.Status != "fail" || !hasReason(got, "independent reviewer identity missing") {
			t.Fatal(got)
		}
	}
}
func TestUnavailableAndDuplicateCheckReferences(t *testing.T) {
	f := fixtureChain()
	f.get("native").Artifact.Status = "unavailable"
	got := f.result()
	if got.Status != "incomplete" || !hasReason(got, "artifact unavailable") {
		t.Fatal(got)
	}
	f = fixtureChain()
	f.get("code").Code.Reviews = append(f.get("code").Code.Reviews, "review")
	got = f.result()
	if got.Status != "fail" || !hasReason(got, "invalid manifest") {
		t.Fatal(got)
	}
}
func hasReason(r Result, reason string) bool {
	for _, i := range r.Issues {
		if i.Reason == reason {
			return true
		}
	}
	return false
}

func TestNativeCommitAndTreeWitnessCoverageRemainDistinct(t *testing.T) {
	f := fixtureChain()
	got := f.result()
	if got.Status != "pass" || got.Coverage["session_code"] != "native_tool_result_tree" {
		t.Fatalf("tree witness: %+v", got)
	}
	f.artifact("native", []byte(strings.Replace(string(f.files["native.json"]), "tested tree "+f.get("code").Code.Tree, "implementation commit "+f.get("code").Code.Commit, 1)))
	got = f.result()
	if got.Status != "pass" || got.Coverage["session_code"] != "native_tool_result_commit" {
		t.Fatalf("commit witness: %+v", got)
	}
}

func TestNativeCustomToolContentWitness(t *testing.T) {
	f := fixtureChain()
	native := string(f.files["native.json"])
	var changed []string
	for _, line := range strings.Split(strings.TrimSuffix(native, "\n"), "\n") {
		var r map[string]any
		_ = json.Unmarshal([]byte(line), &r)
		if r["type"] == "response_item" {
			p := r["payload"].(map[string]any)
			p["type"] = "custom_tool_call_output"
			p["output"] = []map[string]string{{"type": "input_text", "text": "test metadata"}, {"type": "input_text", "text": "IMPLEMENTATION_COMMIT " + f.get("code").Code.Commit}}
			b, _ := json.Marshal(r)
			line = string(b)
		}
		changed = append(changed, line)
	}
	f.artifact("native", []byte(strings.Join(changed, "\n")+"\n"))
	got := f.result()
	if got.Status != "pass" || got.Coverage["session_code"] != "native_tool_result_commit" {
		t.Fatalf("real custom-tool shape not joined: %+v", got)
	}
}
