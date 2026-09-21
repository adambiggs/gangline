package main

import (
	"testing"

	"github.com/adambiggs/gangline/harness"
)

func TestApplyLaunchPolicyKeepsOperatorArgumentsOutOfCollar(t *testing.T) {
	command := harness.Command{Name: "codex", Args: []string{"-m", "gpt"}}
	settings := settings{LaunchArgs: map[string][]string{
		"codex": {"--dangerously-bypass-approvals-and-sandbox"},
	}}
	got := applyLaunchPolicy(command, "codex", settings)
	want := []string{"-m", "gpt", "--dangerously-bypass-approvals-and-sandbox"}
	if len(got.Args) != len(want) {
		t.Fatalf("args = %q, want %q", got.Args, want)
	}
	for index := range want {
		if got.Args[index] != want[index] {
			t.Fatalf("args = %q, want %q", got.Args, want)
		}
	}
	if len(command.Args) != 2 {
		t.Fatalf("input command mutated: %#v", command)
	}
}
