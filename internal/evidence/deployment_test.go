package evidence

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func fixtureDeployment() (*fixture, DeploymentReport) {
	f := fixtureChain()
	f.m.Deployment = "installed"
	f.add("installed", "deployment", &Deployment{Code: "code", Result: "readback"})
	build := BuildIdentity{Commit: f.get("code").Code.Commit, SHA256: strings.Repeat("b", 64)}
	zero := 0
	d := DeploymentReport{Version: 1, ID: "installed", Assignment: "assignment", Repository: "example/project", ObservedAt: "2026-11-02T02:00:00Z", Candidate: build, Installed: build, Acceptance: DeploymentAcceptance{Build: build, Status: "pass", ExitCode: &zero}, Report: "host-report", Review: "readback-review"}
	f.artifact("host-report", []byte("retained host result"))
	f.artifact("readback-review", []byte("retained review"))
	f.json("readback", d)
	return f, d
}

func deploymentResult(f *fixture) Result {
	return Verify(f.m, "assignment", "deployment", func(path string, _ int64) ([]byte, error) {
		b, ok := f.files[path]
		if !ok {
			return nil, fmt.Errorf("missing")
		}
		return b, nil
	})
}

func TestDeploymentJoinsThroughCIAndRetainedReadback(t *testing.T) {
	f, d := fixtureDeployment()
	r := deploymentResult(f)
	if r.Status != "pass" || r.Deployment == nil || r.Deployment.Installed != d.Installed || r.Commit != d.Installed.Commit || r.Coverage["deployment"] != "matching_retained_readback" || len(r.Checks) != 2 {
		t.Fatalf("joined view: %+v", r)
	}
	if r.Coverage["authorship"] != "not_authenticated" || r.Coverage["deployment_provenance"] != "reported_identities; not_host_authenticated" {
		t.Fatalf("trust: %v", r.Coverage)
	}
	// Requiring CI alone remains the old view, without claiming deployment.
	if old := f.result(); old.Status != "pass" || old.Deployment != nil || old.Coverage["deployment"] != "not_exercised" {
		t.Fatalf("CI semantics changed: %+v", old)
	}
}

func TestDeploymentRejectsMissingAndMismatchedEvidence(t *testing.T) {
	cases := []struct {
		name, status string
		change       func(*fixture, *DeploymentReport)
	}{
		{"no selected deployment", "incomplete", func(f *fixture, _ *DeploymentReport) { f.m.Deployment = "" }},
		{"wrong code record", "fail", func(f *fixture, _ *DeploymentReport) { f.get("installed").Deployment.Code = "other" }},
		{"missing readback", "incomplete", func(f *fixture, _ *DeploymentReport) { delete(f.files, "readback.json") }},
		{"wrong assignment", "fail", func(_ *fixture, d *DeploymentReport) { d.Assignment = "other" }},
		{"wrong repository", "fail", func(_ *fixture, d *DeploymentReport) { d.Repository = "other/project" }},
		{"newer revision", "fail", func(_ *fixture, d *DeploymentReport) {
			d.Candidate.Commit = strings.Repeat("c", 40)
			d.Installed = d.Candidate
			d.Acceptance.Build = d.Candidate
		}},
		{"rolled back digest", "fail", func(_ *fixture, d *DeploymentReport) { d.Installed.SHA256 = strings.Repeat("c", 64) }},
		{"acceptance on another binary", "fail", func(_ *fixture, d *DeploymentReport) { d.Acceptance.Build.SHA256 = strings.Repeat("c", 64) }},
		{"failed acceptance", "fail", func(_ *fixture, d *DeploymentReport) { d.Acceptance.Status = "fail" }},
		{"missing exit", "incomplete", func(_ *fixture, d *DeploymentReport) { d.Acceptance.ExitCode = nil }},
		{"nonzero exit", "fail", func(_ *fixture, d *DeploymentReport) { n := 1; d.Acceptance.ExitCode = &n }},
		{"unknown acceptance", "incomplete", func(_ *fixture, d *DeploymentReport) { d.Acceptance.Status = "unknown" }},
		{"missing report", "incomplete", func(f *fixture, _ *DeploymentReport) { delete(f.files, "host-report.json") }},
		{"changed review bytes", "fail", func(f *fixture, _ *DeploymentReport) { f.files["readback-review.json"] = []byte("changed") }},
		{"stale CI", "fail", func(f *fixture, _ *DeploymentReport) { f.get("ci").CI.RunID++ }},
		{"failed tests", "fail", func(f *fixture, _ *DeploymentReport) {
			changeJSON(f, "test-result", func(m map[string]any) { m["exit_code"] = 1 })
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f, d := fixtureDeployment()
			tc.change(f, &d)
			if _, exists := f.files["readback.json"]; exists {
				f.json("readback", d)
			}
			r := deploymentResult(f)
			if r.Status != tc.status {
				t.Fatalf("want %s, got %+v", tc.status, r)
			}
		})
	}
}

func TestDeploymentWithheldAndMalformedDoNotExposeContent(t *testing.T) {
	f, d := fixtureDeployment()
	f.get("host-report").Artifact.Status = "withheld"
	delete(f.files, "host-report.json")
	if r := deploymentResult(f); r.Status != "incomplete" {
		t.Fatalf("%+v", r)
	}
	const sentinel = "PRIVATE_RESULT_SENTINEL"
	for _, mutate := range []func(*DeploymentReport){
		func(d *DeploymentReport) { d.Installed.SHA256 = sentinel },
		func(d *DeploymentReport) { d.ObservedAt = sentinel },
		func(d *DeploymentReport) { d.Acceptance.Status = sentinel },
	} {
		f, d = fixtureDeployment()
		mutate(&d)
		f.json("readback", d)
		r := deploymentResult(f)
		b, _ := json.Marshal(r)
		if r.Status != "fail" || strings.Contains(string(b), sentinel) {
			t.Fatalf("malformed result leaked or passed: %s", b)
		}
	}
}
