//go:build acceptance

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/adambiggs/gangline/harness"
)

func TestHitchKeepsLinkedWorktreeGitBehindNativeApproval(t *testing.T) {
	f := newStateFixture(t)
	fakeCodexOnPath(t)
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	worktree := filepath.Join(repo, ".worktrees", "owner")
	for _, args := range [][]string{
		{"init", repo},
		{"-C", repo, "-c", "core.hooksPath=/dev/null", "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "--allow-empty", "-m", "seed"},
		{"-C", repo, "worktree", "add", "-b", "owner", worktree},
	} {
		if output, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %q: %v: %s", args, err, output)
		}
	}
	capture := filepath.Join(root, "launch")
	t.Setenv("GANGLINE_TEST_LAUNCH_CAPTURE", capture)
	script := "#!/bin/sh\n# SPDX-License-Identifier: Apache-2.0\n" + fakeTmuxUTF8 + `
case "$1" in
has-session|list-panes) exit 0 ;;
new-window) printf '%s\n' "$@" > "$GANGLINE_TEST_LAUNCH_CAPTURE"; exit 91 ;;
*) exit 92 ;;
esac
`
	if err := os.WriteFile(f.env["GANG_TMUX"], []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	f.env["GANG_CODEX_PERMISSION_PROFILE"] = "gangline"
	f.env["GANG_LAUNCH_ARGS"] = `{"codex":["-c","approval_policy=\"on-request\"","-c","approvals_reviewer=\"auto_review\""]}`
	// Stop at allocation, after the real hitch path renders its native command.
	if err := f.cmd.hitch([]string{"owner", "-c", "codex", "-d", worktree}); err == nil || !strings.Contains(err.Error(), "spawn pane") {
		t.Fatalf("hitch did not reach the launch barrier: %v", err)
	}
	data, err := os.ReadFile(capture)
	if err != nil {
		t.Fatal(err)
	}
	launch := string(data)
	if strings.Contains(launch, "--add-dir") {
		t.Fatalf("hitch bypassed protected Git metadata with a write grant: %s", launch)
	}
	for _, want := range []string{"default_permissions", "gangline", "on-request", "auto_review"} {
		if !strings.Contains(launch, want) {
			t.Errorf("launch lost operator setting %q: %s", want, launch)
		}
	}
}

func TestApplyLaunchPolicyKeepsOperatorArgumentsOutOfCollar(t *testing.T) {
	command := harness.Command{Name: "codex", Args: []string{"-m", "gpt"}}
	settings := settings{LaunchArgs: map[string][]string{
		"codex": {"--sandbox", "workspace-write"},
	}}
	got := applyLaunchPolicy(command, "codex", settings)
	want := []string{"-m", "gpt", "--sandbox", "workspace-write"}
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

func TestApplyLaunchPolicyUsesExactCollarName(t *testing.T) {
	command := harness.Command{Name: "claude", Args: []string{"--model", "sonnet"}}
	settings := settings{LaunchArgs: map[string][]string{"claude-code": {"--other"}}}
	got := applyLaunchPolicy(command, "claude", settings)
	if len(got.Args) != 2 {
		t.Fatalf("arguments from another collar: %q", got.Args)
	}
	settings.LaunchArgs["claude"] = []string{"--configured"}
	got = applyLaunchPolicy(command, "claude", settings)
	if len(got.Args) != 3 || got.Args[2] != "--configured" {
		t.Fatalf("configured arguments = %q", got.Args)
	}
}

func TestCodexProfileLaunch(t *testing.T) {
	base := []string{"-c", `approval_policy="on-request"`, "--add-dir", "/operator-selected"}
	for _, profile := range []string{"", "gangline"} {
		want := slices.Clone(base)
		if profile != "" {
			want = append(want, "-c", `default_permissions="gangline"`)
		}
		got, err := codexProfileLaunch(base, profile)
		if err != nil || !slices.Equal(got, want) {
			t.Fatalf("profile %q: args = %q, %v; want %q", profile, got, err, want)
		}
	}
	if len(base) != 4 {
		t.Fatalf("input arguments mutated: %q", base)
	}

	for _, tc := range []struct {
		name string
		args []string
	}{
		{"permission override", []string{"-c", `default_permissions="team"`}},
		{"sandbox override", []string{"-c", `sandbox_mode="workspace-write"`}},
		{"standard sandbox", []string{"--sandbox", "workspace-write"}},
		{"attached sandbox", []string{"-sworkspace-write"}},
		{"attached sandbox equals", []string{"-s=workspace-write"}},
		{"config profile", []string{"--profile", "other"}},
		{"attached config profile", []string{"-pother"}},
		{"attached permission override", []string{`-cdefault_permissions="team"`}},
		{"attached permission override equals", []string{`-c=default_permissions="team"`}},
		{"spaced sandbox override", []string{"-c", `sandbox_mode = "workspace-write"`}},
		{"bypass", []string{"--dangerously-bypass-approvals-and-sandbox"}},
		{"bypass alias", []string{"--yolo"}},
		{"config profile override", []string{"-c", `profile="other"`}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := codexProfileLaunch(tc.args, "gangline"); err == nil {
				t.Fatalf("expected refusal for %q", tc.args)
			}
		})
	}
}
