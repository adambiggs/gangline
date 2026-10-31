package harness

import (
	"slices"
	"strings"
	"testing"
)

func TestAlternateCodexResumeKeepsSessionAndHooks(t *testing.T) {
	c, err := EmbeddedCollar("codex")
	if err != nil {
		t.Fatal(err)
	}
	c.Name = "codex-qwen"
	c.Launch.Args = append([]string{"--profile", "qwen-openrouter"}, c.Launch.Args...)
	c.Launch.ResumeArgs = append([]string{"--profile", "qwen-openrouter"}, c.Launch.ResumeArgs...)
	for _, session := range []string{"", "session-id"} {
		command, err := RenderLaunch(c, LaunchOptions{ResumeSession: session, HookCommand: []string{"gang", "hook"}, Model: "qwen/qwen3-coder-next"})
		if err != nil {
			t.Fatal(err)
		}
		joined := strings.Join(command.Args, "\n")
		for _, want := range []string{"--profile\nqwen-openrouter", "hooks.UserPromptSubmit=", "hooks.Stop=", "-m\nqwen/qwen3-coder-next"} {
			if !strings.Contains(joined, want) {
				t.Fatalf("missing %q in %v", want, command.Args)
			}
		}
		if command.Name != "codex" {
			t.Fatalf("wrong executable: %s", command.Name)
		}
		if session != "" && (!slices.Contains(command.Args, "resume") || !slices.Contains(command.Args, session)) {
			t.Fatal("resume lost native session")
		}
	}
}
