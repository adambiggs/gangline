package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/adambiggs/gangline/harness"
)

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

func TestLinkedWorktreeGitdirGrant(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	worktree := filepath.Join(root, "worktree")
	for _, args := range [][]string{
		{"init", repo},
		{"-C", repo, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "--allow-empty", "-m", "seed"},
		{"-C", repo, "worktree", "add", "-b", "test-worktree", worktree},
	} {
		if output, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %q: %v: %s", args, err, output)
		}
	}
	for _, directory := range []string{repo, root} {
		got := linkedWorktreeGitdir(directory)
		if got != "" {
			t.Fatalf("gitdir for %s = %q", directory, got)
		}
	}
	got := linkedWorktreeGitdir(worktree)
	want, err := filepath.EvalSymlinks(filepath.Join(repo, ".git", "worktrees", "worktree"))
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("gitdir = %q, want %q", got, want)
	}
	nested := filepath.Join(worktree, "nested")
	if err := os.Mkdir(nested, 0o700); err != nil {
		t.Fatal(err)
	}
	if fromNested := linkedWorktreeGitdir(nested); fromNested != want {
		t.Fatalf("nested gitdir = %q", fromNested)
	}
	forged := filepath.Join(root, "forged")
	if err := os.Mkdir(forged, 0o700); err != nil {
		t.Fatal(err)
	}
	pointer, err := os.ReadFile(filepath.Join(worktree, ".git"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(forged, ".git"), pointer, 0o600); err != nil {
		t.Fatal(err)
	}
	if fromForged := linkedWorktreeGitdir(forged); fromForged != "" {
		t.Fatalf("forged gitdir = %q", fromForged)
	}
	if _, err := os.Stat(got); err != nil {
		t.Fatal(err)
	}
	if grant := codexGitdirGrant("gangline", got); grant != `permissions.gangline.filesystem={"`+want+`"="write"}` {
		t.Fatalf("grant = %q", grant)
	}
}

func TestCodexProfileLaunch(t *testing.T) {
	gitdir := "/repo/.git/worktrees/worker"
	base := []string{"-c", `approval_policy="on-request"`}
	unchanged, err := codexProfileLaunch(base, "", gitdir)
	if err != nil || len(unchanged) != len(base) {
		t.Fatalf("unset profile changed arguments: %q, %v", unchanged, err)
	}
	got, err := codexProfileLaunch(base, "gangline", gitdir)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"-c", `approval_policy="on-request"`, "-c", `default_permissions="gangline"`, "-c", `permissions.gangline.filesystem={"/repo/.git/worktrees/worker"="write"}`}
	if len(got) != len(want) {
		t.Fatalf("args = %q, want %q", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("args = %q, want %q", got, want)
		}
	}
	if len(base) != 2 {
		t.Fatalf("input arguments mutated: %q", base)
	}
	got, err = codexProfileLaunch(nil, "gangline", "")
	if err != nil || len(got) != 2 {
		t.Fatalf("ordinary checkout args = %q, %v", got, err)
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
			if _, err := codexProfileLaunch(tc.args, "gangline", gitdir); err == nil {
				t.Fatalf("expected refusal for %q", tc.args)
			}
		})
	}
}
