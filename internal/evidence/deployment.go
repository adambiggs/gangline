package evidence

import "time"

// Deployment joins one code record to a retained, explicitly imported readback.
type Deployment struct {
	Code   string `json:"code"`
	Result string `json:"result_artifact"`
}

type BuildIdentity struct {
	Commit string `json:"commit"`
	SHA256 string `json:"sha256"`
}

type DeploymentAcceptance struct {
	Build    BuildIdentity `json:"build"`
	Status   string        `json:"status"`
	ExitCode *int          `json:"exit_code"`
}

// DeploymentReport is a producer's export or a manual import of a retained
// host result. Report and Review retain its supporting bytes; hashing them
// does not authenticate the import or machine-interpret a prose review.
type DeploymentReport struct {
	Version    int                  `json:"version"`
	ID         string               `json:"id"`
	Assignment string               `json:"assignment"`
	Repository string               `json:"repository"`
	ObservedAt string               `json:"observed_at"`
	Candidate  BuildIdentity        `json:"candidate"`
	Installed  BuildIdentity        `json:"installed"`
	Acceptance DeploymentAcceptance `json:"acceptance"`
	Report     string               `json:"report_artifact"`
	Review     string               `json:"review_artifact"`
}

type DeploymentView struct {
	Record     string               `json:"record"`
	Status     string               `json:"status"`
	ObservedAt string               `json:"observed_at"`
	Candidate  BuildIdentity        `json:"candidate"`
	Installed  BuildIdentity        `json:"installed"`
	Acceptance DeploymentAcceptance `json:"acceptance"`
}

func (v *verifier) deployment(id, code string, c *Code, assignment string) {
	v.result.Coverage["deployment"] = "incomplete"
	r, ok := v.record(id, "deployment")
	if !ok {
		return
	}
	if r.Deployment.Code != code {
		v.issue(id, "fail", "deployment targets different code record")
		return
	}
	b, ok := v.artifact(r.Deployment.Result)
	if !ok {
		return
	}
	var d DeploymentReport
	if strict(b, &d) != nil || d.Version != 1 {
		v.issue(id, "fail", "invalid structured deployment result")
		return
	}
	// Only validated identity/enum fields reach the view; never echo malformed
	// input values, unknown host fields or command output.
	for _, build := range []BuildIdentity{d.Candidate, d.Installed, d.Acceptance.Build} {
		if !gitID.MatchString(build.Commit) || !digest.MatchString(build.SHA256) {
			v.issue(id, "fail", "invalid deployment build identity")
			return
		}
	}
	if _, err := time.Parse(time.RFC3339Nano, d.ObservedAt); err != nil {
		v.issue(id, "fail", "invalid deployment observation time")
		return
	}
	if d.Acceptance.Status != "pass" && d.Acceptance.Status != "fail" && d.Acceptance.Status != "unknown" {
		v.issue(id, "fail", "invalid deployment acceptance status")
		return
	}
	view := &DeploymentView{Record: id, Status: "pass", ObservedAt: d.ObservedAt, Candidate: d.Candidate, Installed: d.Installed, Acceptance: d.Acceptance}
	v.result.Deployment = view
	before := len(v.result.Issues)
	if d.ID != id || d.Assignment != assignment || d.Repository != c.Repository {
		v.issue(id, "fail", "deployment assignment or repository identity mismatch")
	}
	if d.Candidate.Commit != c.Commit || d.Installed != d.Candidate || d.Acceptance.Build != d.Installed {
		v.issue(id, "fail", "candidate, installed or accepted build does not match tested code")
	}
	if d.Acceptance.Status == "unknown" || d.Acceptance.ExitCode == nil {
		v.issue(id, "incomplete", "deployment acceptance outcome unavailable")
	} else if d.Acceptance.Status != "pass" || *d.Acceptance.ExitCode != 0 {
		v.issue(id, "fail", "installed acceptance did not succeed")
	}
	_, reportOK := v.artifact(d.Report)
	_, reviewOK := v.artifact(d.Review)
	if !reportOK || !reviewOK {
		view.Status = "incomplete"
	}
	for _, issue := range v.result.Issues[before:] {
		if issue.Status == "fail" || view.Status == "pass" {
			view.Status = issue.Status
		}
	}
	if view.Status == "pass" {
		v.result.Coverage["deployment"] = "matching_retained_readback"
	} else {
		v.result.Coverage["deployment"] = view.Status
	}
	v.result.Coverage["deployment_provenance"] = "reported_identities; not_host_authenticated"
	v.result.Coverage["deployment_review"] = "retained_bytes; not_machine_interpreted"
}
