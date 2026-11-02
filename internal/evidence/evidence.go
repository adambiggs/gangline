// Package evidence verifies explicitly supplied evidence without collecting it.
package evidence

import (
	"bytes"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/adambiggs/gangline/harness"
)

// Manifest is a private, explicitly imported assignment's evidence. Paths are
// relative to the manifest, not the live team's state or a native home directory.
type Manifest struct {
	Required *Requirements `json:"required_checks,omitempty"`
	Version  int           `json:"version"`
	Scope    Scope         `json:"scope"`
	Target   string        `json:"target_code,omitempty"`
	Records  []Record      `json:"records"`
}
type Requirements struct {
	Tests [][]string `json:"tests,omitempty"`
	Jobs  []string   `json:"jobs,omitempty"`
}

type Scope struct {
	Host       string `json:"host"`
	Team       string `json:"team_instance"`
	Hitch      string `json:"hitch"`
	Assignment string `json:"assignment"`
}
type Record struct {
	ID         string    `json:"id"`
	Kind       string    `json:"kind"`
	Provenance string    `json:"provenance"`
	Artifact   *Artifact `json:"artifact,omitempty"`
	Run        *Run      `json:"run,omitempty"`
	Test       *Test     `json:"test,omitempty"`
	Code       *Code     `json:"code,omitempty"`
	Review     *Review   `json:"review,omitempty"`
	CI         *CI       `json:"ci,omitempty"`
	Usage      *Usage    `json:"usage,omitempty"`
}
type Artifact struct {
	Path       string `json:"path,omitempty"`
	SHA256     string `json:"sha256,omitempty"`
	Size       int64  `json:"size"`
	Status     string `json:"status"`
	Checkpoint string `json:"checkpoint,omitempty"`
}
type Run struct {
	Native          string `json:"native_artifact"`
	Harness         string `json:"harness"`
	Session         string `json:"session"`
	Turn            string `json:"turn"`
	Implementer     string `json:"implementer"`
	RequestedModel  string `json:"requested_model,omitempty"`
	RequestedEffort string `json:"requested_effort,omitempty"`
}
type Test struct {
	Run    string `json:"run"`
	Result string `json:"result_artifact"`
}
type Code struct {
	NativeCall   string   `json:"native_call"`
	Run          string   `json:"run"`
	Repository   string   `json:"repository"`
	Commit       string   `json:"commit"`
	CommitObject string   `json:"commit_artifact"`
	Tree         string   `json:"tree"`
	Tests        []string `json:"tests"`
	Reviews      []string `json:"reviews"`
	CI           []string `json:"ci"`
}
type Review struct {
	Result string `json:"result_artifact"`
}
type CI struct {
	Jobs         string   `json:"jobs_artifact"`
	RunID        int64    `json:"run_id"`
	Attempt      int      `json:"attempt"`
	RequiredJobs []string `json:"required_jobs"`
}

// TestResult is emitted by the actual test runner, not extracted from its prose.
// InputTree must include all relevant tracked and untracked test inputs.
type TestResult struct {
	Version   int      `json:"version"`
	ID        string   `json:"id"`
	Run       string   `json:"run"`
	InputTree string   `json:"input_tree"`
	Argv      []string `json:"argv"`
	CWD       string   `json:"cwd"`
	ExitCode  *int     `json:"exit_code"`
	Outcome   string   `json:"outcome"`
	Output    string   `json:"output_artifact"`
}

// ReviewResult records an independent review's exact source target. Importing
// this attestation cannot authenticate who wrote it; that is the local trust boundary.
type ReviewResult struct {
	Version    int      `json:"version"`
	ID         string   `json:"id"`
	Repository string   `json:"repository"`
	Commit     string   `json:"commit"`
	Reviewer   string   `json:"reviewer"`
	Verdict    string   `json:"verdict"`
	Unresolved []string `json:"unresolved"`
	Report     string   `json:"report_artifact"`
}
type Issue struct {
	Record string `json:"record,omitempty"`
	Status string `json:"status"`
	Reason string `json:"reason"`
}
type Check struct {
	ExitCode *int     `json:"exit_code,omitempty"`
	Record   string   `json:"record"`
	Kind     string   `json:"kind"`
	Argv     []string `json:"argv,omitempty"`
	CWD      string   `json:"cwd,omitempty"`
	RunID    int64    `json:"run_id,omitempty"`
	Attempt  int      `json:"attempt,omitempty"`
	Job      string   `json:"job,omitempty"`
	Status   string   `json:"status"`
}
type Result struct {
	Usage      *UsageResult      `json:"usage,omitempty"`
	Checks     []Check           `json:"checks"`
	Status     string            `json:"status"`
	Assignment string            `json:"assignment"`
	Stage      string            `json:"stage"`
	Code       string            `json:"code,omitempty"`
	Commit     string            `json:"commit,omitempty"`
	Tree       string            `json:"tree,omitempty"`
	Issues     []Issue           `json:"issues"`
	Sources    []Source          `json:"sources"`
	Coverage   map[string]string `json:"coverage"`
	Trust      string            `json:"trust"`
}
type Source struct {
	Provenance         string `json:"provenance"`
	CheckpointVerified bool   `json:"checkpoint_verified"`
	Record             string `json:"record"`
	SHA256             string `json:"sha256"`
	Size               int64  `json:"size"`
	Path               string `json:"path"`
	Checkpoint         string `json:"checkpoint,omitempty"`
}

var identifier = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,199}$`)
var digest = regexp.MustCompile(`^[a-f0-9]{64}$`)
var gitID = regexp.MustCompile(`^[a-f0-9]{40}$`)

// Decode rejects unsupported syntax without echoing private input in errors.
func Decode(input []byte) (Manifest, error) {
	var m Manifest
	if err := strict(input, &m); err != nil {
		return m, errors.New("invalid evidence manifest")
	}
	if m.Version != 1 {
		return m, errors.New("unsupported evidence manifest version")
	}
	for _, id := range []string{m.Scope.Host, m.Scope.Team, m.Scope.Hitch, m.Scope.Assignment} {
		if !identifier.MatchString(id) {
			return m, errors.New("invalid evidence identity")
		}
	}
	if m.Target != "" && !identifier.MatchString(m.Target) {
		return m, errors.New("invalid evidence identity")
	}
	seen := map[string][]byte{}
	records := make([]Record, 0, len(m.Records))
	for _, r := range m.Records {
		if !identifier.MatchString(r.ID) {
			return m, errors.New("invalid record identity")
		}
		if r.Provenance != "manual_import" && r.Provenance != "producer_export" {
			return m, errors.New("record provenance required")
		}
		count := 0
		for _, yes := range []bool{r.Artifact != nil, r.Run != nil, r.Test != nil, r.Code != nil, r.Review != nil, r.CI != nil, r.Usage != nil} {
			if yes {
				count++
			}
		}
		valid := r.Kind == "artifact" && r.Artifact != nil || r.Kind == "run" && r.Run != nil || r.Kind == "test" && r.Test != nil || r.Kind == "code" && r.Code != nil || r.Kind == "review" && r.Review != nil || r.Kind == "ci" && r.CI != nil || r.Kind == "usage" && r.Usage != nil
		if count != 1 || !valid {
			return m, errors.New("unsupported or inconsistent record kind")
		}
		b, _ := json.Marshal(r)
		if previous, ok := seen[r.ID]; ok {
			if !bytes.Equal(previous, b) {
				return m, errors.New("conflicting record identity")
			}
			continue
		}
		if r.Code != nil {
			for _, list := range [][]string{r.Code.Tests, r.Code.Reviews, r.Code.CI} {
				seenIDs := map[string]bool{}
				for _, id := range list {
					if seenIDs[id] {
						return m, errors.New("duplicate check reference")
					}
					seenIDs[id] = true
				}
			}
		}
		seen[r.ID] = b
		records = append(records, r)
	}
	m.Records = records
	return m, nil
}
func strict(data []byte, v any) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		return err
	}
	if d.Decode(new(any)) != io.EOF {
		return errors.New("trailing data")
	}
	return nil
}

type verifier struct {
	result  Result
	records map[string]Record
	read    func(string, int64) ([]byte, error)
	data    map[string][]byte
	checked map[string]bool
}

func (v *verifier) issue(id, status, reason string) {
	if !identifier.MatchString(id) {
		id = ""
	}
	v.result.Issues = append(v.result.Issues, Issue{id, status, reason})
	if status == "fail" || v.result.Status == "pass" {
		v.result.Status = status
	}
}
func (v *verifier) record(id, kind string) (Record, bool) {
	r, ok := v.records[id]
	if !ok || r.Kind != kind {
		v.issue(id, "incomplete", "required "+kind+" record missing")
		return Record{}, false
	}
	return r, true
}
func (v *verifier) artifact(id string) ([]byte, bool) {
	if v.checked[id] {
		b, ok := v.data[id]
		return b, ok
	}
	v.checked[id] = true
	r, ok := v.record(id, "artifact")
	if !ok {
		return nil, false
	}
	a := r.Artifact
	if a.Status == "withheld" || a.Status == "unavailable" {
		v.issue(id, "incomplete", "artifact "+a.Status)
		return nil, false
	}
	if a.Status != "available" || !digest.MatchString(a.SHA256) || a.Size < 0 || a.Size == int64(^uint64(0)>>1) || a.Path == "" {
		v.issue(id, "fail", "invalid artifact metadata")
		return nil, false
	}
	b, err := v.read(a.Path, a.Size)
	if err != nil {
		v.issue(id, "incomplete", "artifact unreadable")
		return nil, false
	}
	sum := sha256.Sum256(b)
	if int64(len(b)) != a.Size || hex.EncodeToString(sum[:]) != a.SHA256 {
		v.issue(id, "fail", "artifact bytes do not match digest and size")
		return nil, false
	}
	v.data[id] = b
	v.result.Sources = append(v.result.Sources, Source{Provenance: r.Provenance, Record: id, SHA256: a.SHA256, Size: a.Size, Path: a.Path, Checkpoint: a.Checkpoint})
	return b, true
}

// Verify reads only the declared target's evidence. read must confine access to
// the explicit input directory; it must not search live state for missing data.
func Verify(m Manifest, assignment, stage string, read func(string, int64) ([]byte, error)) Result {
	v := verifier{result: Result{Status: "pass", Assignment: m.Scope.Assignment, Stage: stage, Issues: []Issue{}, Sources: []Source{}, Checks: []Check{}, Coverage: map[string]string{
		"deployment": "not_exercised", "cost": "unavailable", "retention": "unavailable", "local_restore": "not_exercised", "backend_effort": "unavailable", "loaded_bytes": "unavailable", "authorship": "not_authenticated", "required_checks": "caller_declared", "review_independence": "identity_strings_only", "session_code": "unavailable",
	}, Trust: "explicit local imports; integrity and joins do not authenticate source assertions"}, records: map[string]Record{}, read: read, data: map[string][]byte{}, checked: map[string]bool{}}
	encoded, err := json.Marshal(m)
	if err != nil {
		v.issue("", "fail", "invalid manifest")
		return v.result
	}
	validated, err := Decode(encoded)
	if err != nil {
		v.issue("", "fail", "invalid manifest")
		return v.result
	}
	m = validated
	if assignment != m.Scope.Assignment {
		v.issue("", "fail", "assignment mismatch")
		return v.result
	}
	if stage != "ci" && stage != "usage" {
		v.issue("", "not_exercised", "requested stage is not implemented")
		return v.result
	}
	for _, r := range m.Records {
		v.records[r.ID] = r
	}
	if stage == "usage" {
		v.usage(m.Records, m.Scope.Hitch)
		return v.result
	}
	target, ok := v.record(m.Target, "code")
	if !ok {
		return v.result
	}
	c := target.Code
	v.result.Code = target.ID
	if !gitID.MatchString(c.Commit) || !gitID.MatchString(c.Tree) || c.Repository == "" {
		v.issue(target.ID, "fail", "invalid code identity")
		return v.result
	}
	v.result.Commit = c.Commit
	v.result.Tree = c.Tree
	v.code(target.ID, c)
	run, ok := v.record(c.Run, "run")
	if ok {
		v.run(run.ID, run.Run)
		v.witness(target.ID, c, run.Run)
	}
	if len(c.Tests) == 0 {
		v.issue(target.ID, "incomplete", "tested tree evidence missing")
	}
	for _, id := range c.Tests {
		v.test(id, c)
	}
	if len(c.Reviews) == 0 {
		v.issue(target.ID, "incomplete", "independent review missing")
	}
	for _, id := range c.Reviews {
		v.review(id, c, run.Run)
	}
	if len(c.CI) == 0 {
		v.issue(target.ID, "incomplete", "hosted CI evidence missing")
	}
	for _, id := range c.CI {
		v.ci(id, c)
	}
	if m.Required != nil && (len(m.Required.Tests) > 0 || len(m.Required.Jobs) > 0) {
		v.result.Coverage["required_checks"] = "explicit_manifest_policy"
		for _, argv := range m.Required.Tests {
			found := false
			for _, check := range v.result.Checks {
				if check.Kind == "test" && sameArgs(check.Argv, argv) {
					found = true
				}
			}
			if !found {
				v.issue(target.ID, "incomplete", "required test command absent")
			}
		}
		for _, job := range m.Required.Jobs {
			found := false
			for _, check := range v.result.Checks {
				if check.Kind == "ci" && check.Job == job {
					found = true
				}
			}
			if !found {
				v.issue(target.ID, "incomplete", "required policy CI job absent")
			}
		}
	}
	return v.result
}
func sameArgs(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return len(a) > 0
}
func (v *verifier) code(id string, c *Code) {
	b, ok := v.artifact(c.CommitObject)
	if !ok {
		return
	}
	hash := sha1.New()
	fmt.Fprintf(hash, "commit %d\x00", len(b))
	hash.Write(b)
	if hex.EncodeToString(hash.Sum(nil)) != c.Commit || !bytes.HasPrefix(b, []byte("tree "+c.Tree+"\n")) {
		v.issue(id, "fail", "Git commit object does not match commit and tested tree")
	}
}
func (v *verifier) run(id string, r *Run) {
	if r.Harness != "codex" {
		v.issue(id, "incomplete", "native format not supported by existing reader")
		return
	}
	if r.Session == "" || r.Turn == "" || r.Implementer == "" {
		v.issue(id, "incomplete", "native run identity missing")
		return
	}
	b, ok := v.artifact(r.Native)
	if !ok {
		return
	}
	t, err := harness.ReadTranscript(harness.Invocation{Name: "codex-session-log"}, bytes.NewReader(b), r.Session, 0, time.Time{})
	if err != nil {
		v.issue(id, "fail", "native transcript identity or records invalid")
		return
	}
	if t.Offset != int64(len(b)) {
		v.issue(id, "incomplete", "native transcript has a partial final record")
	}
	found := false
	model := "unavailable"
	for _, x := range t.Readings {
		if x.Kind == "model" {
			model = x.Model
		}
		if x.NativeEvent == "task_complete" && x.TurnID == r.Turn {
			found = true
			v.result.Coverage["observed_model"] = model
			if x.Kind != "turn-finished" {
				v.issue(id, "fail", "native turn ended with failure")
			}
		}
	}
	if !found {
		v.issue(id, "incomplete", "native successful terminal boundary missing")
	}
	if r.RequestedModel != "" {
		v.result.Coverage["requested_model"] = r.RequestedModel
	} else {
		v.result.Coverage["requested_model"] = "unavailable"
	}
	if r.RequestedEffort != "" {
		v.result.Coverage["requested_effort"] = r.RequestedEffort
	} else {
		v.result.Coverage["requested_effort"] = "unavailable"
	}
}
func (v *verifier) test(id string, c *Code) {
	r, ok := v.record(id, "test")
	if !ok {
		return
	}
	b, ok := v.artifact(r.Test.Result)
	if !ok {
		return
	}
	var t TestResult
	if strict(b, &t) != nil || t.Version != 1 {
		v.issue(id, "fail", "invalid structured test result")
		return
	}
	v.result.Checks = append(v.result.Checks, Check{Record: id, Kind: "test", Argv: t.Argv, CWD: t.CWD, Status: t.Outcome, ExitCode: t.ExitCode})
	if t.ID != id || t.Run != r.Test.Run || t.Run != c.Run || t.InputTree != c.Tree {
		v.issue(id, "fail", "test identity or input tree mismatch")
	}
	if t.ExitCode == nil || t.Outcome == "unknown" {
		v.issue(id, "incomplete", "test exit status unavailable")
	} else if *t.ExitCode != 0 || t.Outcome != "exited" {
		v.issue(id, "fail", "test process did not succeed")
	}
	if len(t.Argv) == 0 || t.CWD == "" {
		v.issue(id, "incomplete", "test command identity missing")
	}
	v.artifact(t.Output)
}
func (v *verifier) review(id string, c *Code, run *Run) {
	r, ok := v.record(id, "review")
	if !ok {
		return
	}
	b, ok := v.artifact(r.Review.Result)
	if !ok {
		return
	}
	var rr ReviewResult
	if strict(b, &rr) != nil || rr.Version != 1 {
		v.issue(id, "fail", "invalid review result")
		return
	}
	if rr.ID != id || rr.Repository != c.Repository || rr.Commit != c.Commit {
		v.issue(id, "fail", "review targets different code")
	}
	if strings.TrimSpace(rr.Reviewer) == "" || run == nil || strings.EqualFold(strings.TrimSpace(rr.Reviewer), strings.TrimSpace(run.Implementer)) {
		v.issue(id, "fail", "independent reviewer identity missing")
	}
	if rr.Verdict != "approved" || len(rr.Unresolved) != 0 {
		v.issue(id, "fail", "review has no approval or has unresolved findings")
	}
	v.artifact(rr.Report)
}
func (v *verifier) ci(id string, c *Code) {
	r, ok := v.record(id, "ci")
	if !ok {
		return
	}
	ci := r.CI
	b, ok := v.artifact(ci.Jobs)
	if !ok {
		return
	}
	// The retained GitHub jobs response supplies job/attempt/commit identity.
	var response struct {
		Jobs []struct {
			ID         int64  `json:"id"`
			RunID      int64  `json:"run_id"`
			Attempt    int    `json:"run_attempt"`
			Head       string `json:"head_sha"`
			Name       string `json:"name"`
			Status     string `json:"status"`
			Conclusion string `json:"conclusion"`
			URL        string `json:"html_url"`
		} `json:"jobs"`
	}
	if json.Unmarshal(b, &response) != nil || ci.RunID <= 0 || ci.Attempt <= 0 || len(ci.RequiredJobs) == 0 {
		v.issue(id, "fail", "invalid hosted CI job evidence")
		return
	}
	prefix := "https://github.com/" + c.Repository + "/actions/runs/" + fmt.Sprint(ci.RunID) + "/job/"
	for _, name := range ci.RequiredJobs {
		count := 0
		for _, j := range response.Jobs {
			if j.Name != name {
				continue
			}
			count++
			v.result.Checks = append(v.result.Checks, Check{Record: id, Kind: "ci", RunID: j.RunID, Attempt: j.Attempt, Job: j.Name, Status: j.Conclusion})
			if j.ID <= 0 || j.RunID != ci.RunID || j.Attempt != ci.Attempt || j.Head != c.Commit || !strings.HasPrefix(j.URL, prefix) {
				v.issue(id, "fail", "CI job source identity mismatch")
			}
			if j.Status != "completed" {
				v.issue(id, "incomplete", "required CI job not completed")
			} else if j.Conclusion != "success" {
				v.issue(id, "fail", "required CI job did not succeed")
			}
		}
		if count == 0 {
			v.issue(id, "incomplete", "required CI job absent")
		} else if count > 1 {
			v.issue(id, "fail", "ambiguous CI job attempt")
		}
	}
}

// witness joins code to an observed native tool result in the selected turn.
// This is a reference check, not a second capture adapter or authorship proof.
func (v *verifier) witness(id string, c *Code, r *Run) {
	b, ok := v.artifact(r.Native)
	if !ok {
		return
	}
	active := ""
	for _, line := range bytes.Split(b, []byte("\n")) {
		var entry struct {
			Type    string `json:"type"`
			Payload struct {
				Type   string          `json:"type"`
				Turn   string          `json:"turn_id"`
				Call   string          `json:"call_id"`
				Output json.RawMessage `json:"output"`
			} `json:"payload"`
		}
		if json.Unmarshal(line, &entry) != nil {
			continue
		}
		p := entry.Payload
		if entry.Type == "event_msg" && p.Type == "task_started" {
			active = p.Turn
		}
		if active == r.Turn && c.NativeCall != "" && entry.Type == "response_item" && (p.Type == "function_call_output" || p.Type == "custom_tool_call_output") && p.Call == c.NativeCall {
			output, ok := nativeToolText(p.Output)
			if ok {
				for _, token := range strings.FieldsFunc(output, func(r rune) bool { return !((r >= 'a' && r <= 'f') || (r >= '0' && r <= '9')) }) {
					if token == c.Commit {
						v.result.Coverage["session_code"] = "native_tool_result_commit"
						return
					}
					if token == c.Tree {
						v.result.Coverage["session_code"] = "native_tool_result_tree"
						return
					}
				}
			}
		}
		if entry.Type == "event_msg" && (p.Type == "task_complete" || p.Type == "turn_aborted") {
			active = ""
		}
	}
	v.issue(id, "incomplete", "native tool result does not witness code identity in selected turn")
}

// Codex stores function results as strings and custom-tool results as typed
// input-text blocks. Other content types do not establish a textual witness.
func nativeToolText(raw json.RawMessage) (string, bool) {
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return text, true
	}
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &blocks) != nil {
		return "", false
	}
	var parts []string
	for _, block := range blocks {
		if block.Type == "input_text" {
			parts = append(parts, block.Text)
		}
	}
	return strings.Join(parts, "\n"), len(parts) > 0
}
