package acceptance

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrePushDelegatesGlobalHook(t *testing.T) {
	hook, err := filepath.Abs("../.githooks/pre-push")
	if err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []string{"configured", "fallback", "refusal", "recursion", "callback"} {
		t.Run(scenario, func(t *testing.T) {
			root := t.TempDir()
			config := filepath.Join(root, "global-config")
			hooks := filepath.Join(root, "config", "git", "hooks")
			if err := os.MkdirAll(hooks, 0o700); err != nil {
				t.Fatal(err)
			}
			write := func(path, body string, mode os.FileMode) {
				t.Helper()
				if err := os.WriteFile(path, []byte(body), mode); err != nil {
					t.Fatal(err)
				}
			}
			body := ""
			if scenario != "fallback" {
				globalDirectory := hooks
				if scenario == "recursion" {
					globalDirectory = filepath.Dir(hook)
				}
				body = fmt.Sprintf("[core]\n hooksPath = %s\n", globalDirectory)
			}
			write(config, body, 0o600)
			status := 0
			if scenario == "refusal" {
				status = 23
			}
			write(filepath.Join(hooks, "pre-push"), fmt.Sprintf("#!/bin/sh\nset -eu\nprintf '%%s\\n' \"$@\" > \"$HOOK_FIXTURE/args\"\ncat > \"$HOOK_FIXTURE/refs\"\nexit %d\n", status), 0o700)
			if scenario == "callback" {
				write(filepath.Join(hooks, "pre-push"), "#!/bin/sh\nset -eu\nprintf '%s\\n' \"$@\" > \"$HOOK_FIXTURE/args\"\ncat > \"$HOOK_FIXTURE/refs\"\nprintf 'call\\n' >> \"$HOOK_FIXTURE/calls\"\n[ \"${HOOK_DEPTH:-0}\" -lt 2 ] || exit 97\nexport HOOK_DEPTH=$(( ${HOOK_DEPTH:-0} + 1 ))\nbash \"$HOOK_ENTRY\" \"$@\" < \"$HOOK_FIXTURE/refs\"\n", 0o700)
			}
			env := []string{}
			for _, entry := range os.Environ() {
				if !strings.HasPrefix(entry, "GIT_") && !strings.HasPrefix(entry, "XDG_CONFIG_HOME=") {
					env = append(env, entry)
				}
			}
			env = append(env, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+config, "XDG_CONFIG_HOME="+filepath.Join(root, "config"), "HOOK_FIXTURE="+root, "HOOK_ENTRY="+hook)
			init := exec.Command("git", "init", "--quiet", root)
			init.Env = env
			if output, err := init.CombinedOutput(); err != nil {
				t.Fatalf("initialize fixture: %v\n%s", err, output)
			}
			refs := "(delete) " + strings.Repeat("0", 40) + " refs/heads/obsolete " + strings.Repeat("1", 40) + "\n"
			command := exec.Command("bash", hook, "origin", "https://example.invalid/repo.git")
			command.Dir, command.Env = root, env
			command.Stdin = strings.NewReader(refs)
			output, runErr := command.CombinedOutput()
			if scenario == "recursion" {
				if runErr == nil || !strings.Contains(string(output), "refusing recursive delegation") {
					t.Fatalf("recursive hook result: %v\n%s", runErr, output)
				}
				return
			}
			if command.ProcessState.ExitCode() != status {
				t.Fatalf("hook exit=%d, want %d: %v\n%s", command.ProcessState.ExitCode(), status, runErr, output)
			}
			for name, want := range map[string]string{"args": "origin\nhttps://example.invalid/repo.git\n", "refs": refs} {
				data, err := os.ReadFile(filepath.Join(root, name))
				if err != nil || string(data) != want {
					t.Fatalf("global hook %s=%q, want %q: %v", name, data, want, err)
				}
			}
			if scenario == "refusal" && strings.Contains(string(output), "Go checks") {
				t.Fatalf("repository checks ran after global refusal: %s", output)
			}
			if scenario == "callback" {
				calls, err := os.ReadFile(filepath.Join(root, "calls"))
				if err != nil || string(calls) != "call\n" {
					t.Fatalf("global callback recursed: %q, %v", calls, err)
				}
			}
		})
	}
}
