package harness

import (
	"strings"
	"testing"
)

func TestRenderLaunchInstallsHooksAndOptions(t *testing.T) {
	collar, err := EmbeddedCollar("codex")
	if err != nil {
		t.Fatal(err)
	}
	command, err := RenderLaunch(collar, LaunchOptions{
		HookCommand: []string{"/opt/Gang Line/gang", "hook"},
		Model:       "gpt-test",
		Effort:      "high",
	})
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(command.Args, "\n")
	for _, want := range []string{
		"hooks.Stop=", `command = "'/opt/Gang Line/gang' hook"`,
		"-m\ngpt-test", "-c\nmodel_reasoning_effort=high",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("launch args do not contain %q:\n%s", want, joined)
		}
	}
}

func TestRenderLaunchResume(t *testing.T) {
	collar, err := EmbeddedCollar("claude-code")
	if err != nil {
		t.Fatal(err)
	}
	command, err := RenderLaunch(collar, LaunchOptions{
		ResumeSession: "session-1",
		HookCommand:   []string{"gang", "hook"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(command.Args) < 2 || command.Args[0] != "--resume" || command.Args[1] != "session-1" {
		t.Fatalf("args = %q, want resume prefix", command.Args)
	}
}

func TestRenderLaunchRefusesUnsupportedOption(t *testing.T) {
	collar, err := EmbeddedCollar("codex")
	if err != nil {
		t.Fatal(err)
	}
	_, err = RenderLaunch(collar, LaunchOptions{HookCommand: []string{"gang", "hook"}, RolePrompt: "role"})
	if err == nil {
		t.Fatal("unsupported role prompt was accepted")
	}
}

func TestRenderLaunchRequiresDeclaredHooks(t *testing.T) {
	collar, err := EmbeddedCollar("codex")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := RenderLaunch(collar, LaunchOptions{}); err == nil {
		t.Fatal("hooked collar launched without a hook command")
	}
}
