package harness

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

func TestCheckReportRequiresEveryProbe(t *testing.T) {
	report := CheckReport{Collar: "codex", HarnessVersion: "1.2.3"}
	for _, name := range []string{ProbeLaunch, ProbeTrustPrompt, ProbeHook, ProbeComposer, ProbeSubmit, ProbeTurnBoundary} {
		report.Results = append(report.Results, ProbeResult{Name: name, Passed: true})
	}
	if !report.Passed() {
		t.Fatal("complete passing report did not pass")
	}
	report.Results = append(report.Results[:4], report.Results[5:]...)
	if report.Passed() {
		t.Fatal("report without submit probe passed")
	}
}

func TestCheckReportBuildsReviewableIssueCommand(t *testing.T) {
	report := CheckReport{
		Collar: "codex's fixture", HarnessVersion: "1.2.3 test",
		Results: []ProbeResult{{Name: ProbeComposer, Passed: false, Detail: "no composer; echo unsafe > /dev/null"}},
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
	want := [][]byte{[]byte("issue"), []byte("create"), []byte("--repo"), []byte("owner/repo"), []byte("--title"), []byte("codex's fixture collar is incompatible with 1.2.3 test"), []byte("--body"), []byte(report.IssueBody())}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("issue arguments = %q, want %q", got, want)
	}
}
