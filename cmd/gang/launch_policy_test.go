package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
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
	if grant, err := codexLaunch(nil, "", got); err != nil || !slices.Equal(grant, []string{"--add-dir", want}) {
		t.Fatalf("grant = %q, %v", grant, err)
	}
}

func TestCodexLaunch(t *testing.T) {
	gitdir := "/repo/.git/worktrees/worker"
	base := []string{"-c", `approval_policy="on-request"`}
	for _, tc := range []struct {
		profile, gitdir string
		want            []string
	}{
		{"", "", base},
		{"", gitdir, append(slices.Clone(base), "--add-dir", gitdir)},
		{"gangline", "", append(slices.Clone(base), "-c", `default_permissions="gangline"`)},
		{"gangline", gitdir, append(slices.Clone(base), "-c", `default_permissions="gangline"`, "--add-dir", gitdir)},
	} {
		got, err := codexLaunch(base, tc.profile, tc.gitdir)
		if err != nil || !slices.Equal(got, tc.want) {
			t.Fatalf("profile %q gitdir %q: args = %q, %v; want %q", tc.profile, tc.gitdir, got, err, tc.want)
		}
	}
	if len(base) != 2 {
		t.Fatalf("input arguments mutated: %q", base)
	}
	for _, args := range [][]string{
		{"-s", "read-only"},
		{"--sandbox", "read-only"},
		{"--sandbox=read-only"},
		{"-sread-only"},
		{"-s=read-only"},
		{"-c", `sandbox_mode="read-only"`},
		{"--config", `sandbox_mode = "read-only"`},
		{`-csandbox_mode="read-only"`},
		{`--config=sandbox_mode="read-only"`},
		{"-s", "read-only", "-c", `sandbox_mode="workspace-write"`},
		{"-c", `sandbox_mode="workspace-write"`, "-c", `sandbox_mode="read-only"`},
		{"-c", `sandbox_mode="read-only" # comment`},
		{"-c", `sandbox_mode='read-only'`},
		{"-c", `sandbox_mode=read-only`},
	} {
		if got, err := codexLaunch(args, "", gitdir); err != nil || !slices.Equal(got, args) {
			t.Fatalf("read-only sandbox %q: args = %q, %v", args, got, err)
		}
	}
	for _, args := range [][]string{
		{"-s", "workspace-write"},
		{"-s", "workspace-write", "-c", `sandbox_mode="read-only"`},
		{"-c", `sandbox_mode="read-only"`, "-c", `sandbox_mode="workspace-write"`},
		{"-c", `model="read-only"`},
	} {
		want := append(slices.Clone(args), "--add-dir", gitdir)
		if got, err := codexLaunch(args, "", gitdir); err != nil || !slices.Equal(got, want) {
			t.Fatalf("writable sandbox %q: args = %q, %v; want %q", args, got, err, want)
		}
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
			if _, err := codexLaunch(tc.args, "gangline", gitdir); err == nil {
				t.Fatalf("expected refusal for %q", tc.args)
			}
		})
	}
}
