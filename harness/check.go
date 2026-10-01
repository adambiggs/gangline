package harness

import (
	"fmt"
	"runtime"
	"strings"
)

const (
	ProbeLaunch       = "launch"
	ProbeTrustPrompt  = "trust-prompt"
	ProbeHook         = "hook-fires"
	ProbeComposer     = "composer-detect"
	ProbeSubmit       = "submit"
	ProbeTurnBoundary = "turn-boundary"
)

// ProbeOutcome separates a probe that did not run, or whose instrument failed,
// from one that ran and observed the harness falling short.
type ProbeOutcome int

const (
	ProbeUnknown ProbeOutcome = iota
	ProbeFailed
	ProbePassed
)

func (outcome ProbeOutcome) String() string {
	switch outcome {
	case ProbeFailed:
		return "FAIL"
	case ProbePassed:
		return "PASS"
	default:
		return "UNKNOWN"
	}
}

type ProbeResult struct {
	Name    string
	Outcome ProbeOutcome
	Detail  string
}

type CheckReport struct {
	Collar         string
	HarnessVersion string
	Results        []ProbeResult
}

func RequiredProbes() []string {
	return []string{
		ProbeLaunch,
		ProbeTrustPrompt,
		ProbeHook,
		ProbeComposer,
		ProbeSubmit,
		ProbeTurnBoundary,
	}
}

func (report CheckReport) Passed() bool {
	if len(report.Results) != len(RequiredProbes()) {
		return false
	}
	seen := make(map[string]bool, len(report.Results))
	for _, result := range report.Results {
		if result.Outcome != ProbePassed || seen[result.Name] {
			return false
		}
		seen[result.Name] = true
	}
	for _, name := range RequiredProbes() {
		if !seen[name] {
			return false
		}
	}
	return true
}

// Probes names the reported probes with the given outcome. A required probe
// missing from the report counts as unknown.
func (report CheckReport) Probes(outcome ProbeOutcome) []string {
	var names []string
	reported := make(map[string]bool, len(report.Results))
	for _, result := range report.Results {
		reported[result.Name] = true
		if result.Outcome == outcome {
			names = append(names, result.Name)
		}
	}
	if outcome == ProbeUnknown {
		for _, name := range RequiredProbes() {
			if !reported[name] {
				names = append(names, name)
			}
		}
	}
	return names
}

func (report CheckReport) IssueTitle() string {
	failed := strings.Join(report.Probes(ProbeFailed), ", ")
	if report.HarnessVersion == "" || report.HarnessVersion == "unknown" {
		return fmt.Sprintf("%s collar check failed: %s", report.Collar, failed)
	}
	return fmt.Sprintf("%s collar check failed on %s: %s", report.Collar, report.HarnessVersion, failed)
}

func (report CheckReport) IssueBody() string {
	var body strings.Builder
	fmt.Fprintf(&body, "## Collar check failure\n\n")
	fmt.Fprintf(&body, "- Collar: `%s`\n", report.Collar)
	fmt.Fprintf(&body, "- Harness version: `%s`\n", report.HarnessVersion)
	fmt.Fprintf(&body, "- Platform: `%s/%s`\n\n", runtime.GOOS, runtime.GOARCH)
	body.WriteString("## Probes\n\n")
	for _, result := range report.Results {
		fmt.Fprintf(&body, "- %s `%s`", result.Outcome, result.Name)
		if result.Detail != "" {
			fmt.Fprintf(&body, " — %s", result.Detail)
		}
		body.WriteByte('\n')
	}
	return body.String()
}

func (report CheckReport) IssueCommand(repository string) string {
	return "gh issue create --repo " + shellJoin([]string{repository}) +
		" --title " + shellJoin([]string{report.IssueTitle()}) +
		" --body " + shellJoin([]string{report.IssueBody()})
}
