package acceptance

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrePushChecksCommittedTree(t *testing.T) {
	hook, err := filepath.Abs("../.githooks/pre-push")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	config := filepath.Join(root, "global-config")
	if err := os.WriteFile(config, nil, 0600); err != nil {
		t.Fatal(err)
	}
	var env []string
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "GIT_") && !strings.HasPrefix(entry, "XDG_CONFIG_HOME=") && !strings.HasPrefix(entry, "_GANGLINE_PRE_PUSH_ACTIVE=") {
			env = append(env, entry)
		}
	}
	env = append(env, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+config, "XDG_CONFIG_HOME="+filepath.Join(root, "xdg"))
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
	gate := filepath.Join(root, "test", "go.sh")
	if err := os.WriteFile(gate, []byte("#!/bin/sh\n# SPDX-License-Identifier: Apache-2.0\necho committed-gate-failed\nexit 37\n"), 0700); err != nil {
		t.Fatal(err)
	}
	git("add", "test/go.sh")
	git("-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "--quiet", "-m", "failing pushed tree")
	sha := git("rev-parse", "HEAD")
	if len(sha) != 40 {
		t.Fatalf("commit id = %q", sha)
	}
	if err := os.WriteFile(gate, []byte("#!/bin/sh\n# SPDX-License-Identifier: Apache-2.0\necho working-tree-passed\n"), 0700); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("bash", hook, "origin", "https://example.invalid/repo.git")
	command.Dir, command.Env = root, env
	command.Stdin = strings.NewReader("refs/heads/main " + sha + " refs/heads/main " + strings.Repeat("0", 40) + "\n")
	output, err := command.CombinedOutput()
	if err == nil || command.ProcessState.ExitCode() == 0 || !strings.Contains(string(output), "committed-gate-failed") || !strings.Contains(string(output), "pre-push: refusing") || strings.Contains(string(output), "working-tree-passed") {
		t.Fatalf("pushed-tree hook result: %v\n%s", err, output)
	}
}
