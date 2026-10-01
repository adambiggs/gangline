package harness

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestCheckReportRequiresEveryProbe(t *testing.T) {
	report := CheckReport{Collar: "codex", HarnessVersion: "1.2.3"}
	for _, name := range []string{ProbeLaunch, ProbeTrustPrompt, ProbeHook, ProbeComposer, ProbeSubmit, ProbeTurnBoundary} {
		report.Results = append(report.Results, ProbeResult{Name: name, Outcome: ProbePassed})
	}
	if !report.Passed() {
		t.Fatal("complete passing report did not pass")
	}
	report.Results = append(report.Results[:4], report.Results[5:]...)
	if report.Passed() {
		t.Fatal("report without submit probe passed")
	}
	if got := report.Probes(ProbeUnknown); !reflect.DeepEqual(got, []string{ProbeSubmit}) {
		t.Fatalf("unknown probes = %q, want the missing submit probe", got)
	}
}

func TestCheckReportKeepsUnknownApartFromFailure(t *testing.T) {
	report := CheckReport{Collar: "codex", HarnessVersion: "1.2.3", Results: []ProbeResult{
		{Name: ProbeLaunch, Outcome: ProbePassed},
		{Name: ProbeTrustPrompt, Outcome: ProbeFailed, Detail: "startup not recognized"},
		{Name: ProbeHook, Detail: "probe did not run"},
	}}
	if got := report.Probes(ProbeFailed); !reflect.DeepEqual(got, []string{ProbeTrustPrompt}) {
		t.Fatalf("failed probes = %q", got)
	}
	if got := report.Probes(ProbeUnknown); !reflect.DeepEqual(got, []string{ProbeHook, ProbeComposer, ProbeSubmit, ProbeTurnBoundary}) {
		t.Fatalf("unknown probes = %q", got)
	}
	if got, want := report.IssueTitle(), "codex collar check failed on 1.2.3: trust-prompt"; got != want {
		t.Fatalf("title = %q, want %q", got, want)
	}
	body := report.IssueBody()
	for _, row := range []string{"- PASS `launch`", "- FAIL `trust-prompt` — startup not recognized", "- UNKNOWN `hook-fires` — probe did not run"} {
		if !strings.Contains(body, row) {
			t.Fatalf("body lacks %q:\n%s", row, body)
		}
	}
	if strings.Contains(body, "gang collar check") {
		t.Fatalf("body names the tool that generated it:\n%s", body)
	}
}

func TestCheckReportBuildsReviewableIssueCommand(t *testing.T) {
	report := CheckReport{
		Collar: "codex's fixture", HarnessVersion: "1.2.3 test",
		Results: []ProbeResult{{Name: ProbeComposer, Outcome: ProbeFailed, Detail: "no composer; echo unsafe > /dev/null"}},
	}
	root := t.TempDir()
	program := filepath.Join(root, "gh")
	if err := os.WriteFile(program, []byte("#!/bin/sh\nprintf '%s\\0' \"$@\" > \"$ARGFILE\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	argsFile := filepath.Join(root, "args")
	command := exec.Command("sh", "-c", report.IssueCommand("owner/repo"))
	command.Env = append(os.Environ(), "PATH="+root+":"+os.Getenv("PATH"), "ARGFILE="+argsFile)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("issue command failed: %v\n%s", err, output)
	}
	data, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatal(err)
	}
	got := bytes.Split(bytes.TrimSuffix(data, []byte{0}), []byte{0})
	want := [][]byte{[]byte("issue"), []byte("create"), []byte("--repo"), []byte("owner/repo"), []byte("--title"), []byte("codex's fixture collar check failed on 1.2.3 test: composer-detect"), []byte("--body"), []byte(report.IssueBody())}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("issue arguments = %q, want %q", got, want)
	}
}
