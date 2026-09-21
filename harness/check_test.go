package harness

import (
	"strings"
	"testing"
)

func TestCheckReportRequiresEveryProbe(t *testing.T) {
	report := CheckReport{Collar: "codex", HarnessVersion: "1.2.3"}
	for _, name := range RequiredProbes() {
		report.Results = append(report.Results, ProbeResult{Name: name, Passed: true})
	}
	if !report.Passed() {
		t.Fatal("complete passing report did not pass")
	}
	report.Results = report.Results[:len(report.Results)-1]
	if report.Passed() {
		t.Fatal("incomplete report passed")
	}
}

func TestCheckReportBuildsReviewableIssueCommand(t *testing.T) {
	report := CheckReport{
		Collar: "codex", HarnessVersion: "1.2.3",
		Results: []ProbeResult{{Name: ProbeComposer, Passed: false, Detail: "no composer"}},
	}
	command := report.IssueCommand("owner/repo")
	for _, want := range []string{"gh issue create", "owner/repo", "no composer", "gang collar check"} {
		if !strings.Contains(command, want) {
			t.Fatalf("command does not contain %q: %s", want, command)
		}
	}
}
