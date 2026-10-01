package acceptance

import (
	"bytes"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// prePushFixture commits gate as test/go.sh in a fresh repository whose
// global configuration and temporary directory both live under the test.
func prePushFixture(t *testing.T, gate string) (root, sha string, env []string) {
	t.Helper()
	root = t.TempDir()
	config := filepath.Join(root, "global-config")
	if err := os.WriteFile(config, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "tmp"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "GIT_") && !strings.HasPrefix(entry, "XDG_CONFIG_HOME=") && !strings.HasPrefix(entry, "_GANGLINE_PRE_PUSH_ACTIVE=") && !strings.HasPrefix(entry, "TMPDIR=") {
			env = append(env, entry)
		}
	}
	env = append(env, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+config, "XDG_CONFIG_HOME="+filepath.Join(root, "xdg"), "TMPDIR="+filepath.Join(root, "tmp"), "HOOK_FIXTURE="+root)
	// A fixture may leave read-only directories behind; restore write access
	// so the test's own cleanup can remove them.
	t.Cleanup(func() {
		_ = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err == nil && entry.IsDir() {
				_ = os.Chmod(path, 0700)
			}
			return nil
		})
	})
	git := func(args ...string) string {
		t.Helper()
		command := exec.Command("git", args...)
		command.Dir, command.Env = root, env
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, output)
		}
		return strings.TrimSpace(string(output))
	}
	git("init", "--quiet")
	if err := os.Mkdir(filepath.Join(root, "test"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "test", "go.sh"), []byte("#!/bin/sh\n# SPDX-License-Identifier: Apache-2.0\nset -e\n"+gate), 0700); err != nil {
		t.Fatal(err)
	}
	git("add", "test/go.sh")
	git("-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "--quiet", "-m", "fixture gate")
	return root, git("rev-parse", "HEAD"), env
}

func TestPrePushReportsWorktreeCleanupFailure(t *testing.T) {
	hook, err := filepath.Abs("../.githooks/pre-push")
	if err != nil {
		t.Fatal(err)
	}
	// A locked worktree refuses a single --force removal, so the hook's own
	// cleanup fails while the pushed tree's checks pass. A read-only directory
	// inside it makes the fallback deletion fail as well.
	lock := "git worktree lock --reason fixture-lock \"$PWD\"\n"
	for name, gate := range map[string]string{
		"locked":             lock,
		"locked-undeletable": "mkdir sealed\n: > sealed/file\nchmod 500 sealed\n" + lock,
	} {
		t.Run(name, func(t *testing.T) {
			if name == "locked-undeletable" && os.Geteuid() == 0 {
				t.Skip("root deletes from read-only directories")
			}
			root, sha, env := prePushFixture(t, gate)
			command := exec.Command("bash", hook, "origin", "https://example.invalid/repo.git")
			command.Dir, command.Env = root, env
			command.Stdin = strings.NewReader("refs/heads/main " + sha + " refs/heads/main " + strings.Repeat("0", 40) + "\n")
			output, err := command.CombinedOutput()
			if err != nil {
				t.Fatalf("passing checks with failed cleanup refused the push: %v\n%s", err, output)
			}
			if !strings.Contains(string(output), "pre-push: cannot remove worktree") || !strings.Contains(string(output), "fixture-lock") {
				t.Fatalf("cleanup failure not reported with git's reason:\n%s", output)
			}
			if strings.Count(string(output), "pre-push: cannot remove worktree") != 1 {
				t.Fatalf("cleanup failure reported more than once:\n%s", output)
			}
		})
	}
}

func TestPrePushTerminationRefusesPush(t *testing.T) {
	hook, err := filepath.Abs("../.githooks/pre-push")
	if err != nil {
		t.Fatal(err)
	}
	root, sha, env := prePushFixture(t, "kill -TERM \"$(cat \"$HOOK_FIXTURE/hook.pid\")\"\n")
	command := exec.Command("bash", hook, "origin", "https://example.invalid/repo.git")
	command.Dir, command.Env = root, env
	stdin, err := command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	command.Stdout, command.Stderr = &output, &output
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	// The hook reads its whole ref stream before checking anything, so the pid
	// file exists before the fixture gate can read it.
	if err := os.WriteFile(filepath.Join(root, "hook.pid"), []byte(strconv.Itoa(command.Process.Pid)), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := stdin.Write([]byte("refs/heads/main " + sha + " refs/heads/main " + strings.Repeat("0", 40) + "\n")); err != nil {
		t.Fatal(err)
	}
	if err := stdin.Close(); err != nil {
		t.Fatal(err)
	}
	waitErr := command.Wait()
	if code := command.ProcessState.ExitCode(); code != 143 {
		t.Fatalf("terminated hook exit=%d, want 143: %v\n%s", code, waitErr, output.String())
	}
	if strings.Contains(output.String(), "cannot remove worktree") {
		t.Fatalf("terminated hook reported a cleanup failure that did not happen:\n%s", output.String())
	}
}
