package acceptance

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCommitMessageHookPolicy(t *testing.T) {
	hook, err := filepath.Abs("../.githooks/commit-msg")
	if err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []struct {
		name       string
		message    string
		subject    bool
		wantRefuse bool
		wantReason string
	}{
		{name: "breaking subject", message: "feat!: replace the public command\n", subject: true},
		{name: "release subject", message: "chore(main): release gangline 1.0.0\n", subject: true},
		{name: "merge title", message: "Merge branch main\n", subject: true, wantRefuse: true},
		{name: "fixup title", message: "fixup! fix(installer): build the release\n", subject: true, wantRefuse: true},
		{name: "squash title", message: "squash! fix(installer): build the release\n", subject: true, wantRefuse: true},
		{name: "amend title", message: "amend! fix(installer): build the release\n", subject: true, wantRefuse: true},
		{name: "missing breaking footer", message: "feat!: replace the public command\n", wantRefuse: true, wantReason: "carries no BREAKING CHANGE: footer"},
		{name: "breaking footer", message: "feat!: replace the public command\n\nBREAKING CHANGE: callers must use gang instead.\n"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			messageFile := filepath.Join(t.TempDir(), "message")
			if err := os.WriteFile(messageFile, []byte(scenario.message), 0600); err != nil {
				t.Fatal(err)
			}
			args := []string{messageFile}
			if scenario.subject {
				args = []string{"--subject-only", messageFile}
			}
			output, err := exec.Command(hook, args...).CombinedOutput()
			if scenario.wantRefuse && err == nil {
				t.Fatalf("hook accepted %q: %s", scenario.message, output)
			}
			if !scenario.wantRefuse && err != nil {
				t.Fatalf("hook refused %q: %v\n%s", scenario.message, err, output)
			}
			if scenario.wantRefuse && !strings.Contains(string(output), "commit-msg:") {
				t.Fatalf("hook refusal omitted reason: %s", output)
			}
			if scenario.wantReason != "" && !strings.Contains(string(output), scenario.wantReason) {
				t.Fatalf("hook refusal omitted %q: %s", scenario.wantReason, output)
			}
		})
	}
}
