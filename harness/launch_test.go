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

func TestCodexCompactionBoundaryConfirmsBeforeContinuation(t *testing.T) {
	collar, err := EmbeddedCollar("codex")
	if err != nil {
		t.Fatal(err)
	}
	command, err := RenderLaunch(collar, LaunchOptions{HookCommand: []string{"gang", "hook"}})
	if err != nil {
		t.Fatal(err)
	}
	for event, wantAsync := range map[string]bool{"Stop": true, "PostCompact": false} {
		found := false
		async := false
		for _, arg := range command.Args {
			if strings.HasPrefix(arg, "hooks."+event+"=") {
				found = true
				async = strings.Contains(arg, "async = true")
			}
		}
		if !found || async != wantAsync {
			t.Errorf("%s: found = %v, async = %v; want async = %v", event, found, async, wantAsync)
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

func TestRenderLaunchAddsProbeArgumentsOnlyForProbe(t *testing.T) {
	collar, err := EmbeddedCollar("codex")
	if err != nil {
		t.Fatal(err)
	}
	normal, err := RenderLaunch(collar, LaunchOptions{HookCommand: []string{"gang", "hook"}})
	if err != nil {
		t.Fatal(err)
	}
	probe, err := RenderLaunch(collar, LaunchOptions{HookCommand: []string{"gang", "hook"}, Probe: true})
	if err != nil {
		t.Fatal(err)
	}
	const flag = "--dangerously-bypass-hook-trust"
	if strings.Contains(strings.Join(normal.Args, "\n"), flag) {
		t.Fatalf("ordinary launch contains probe flag: %q", normal.Args)
	}
	if !strings.Contains(strings.Join(probe.Args, "\n"), flag) {
		t.Fatalf("probe launch does not contain probe flag: %q", probe.Args)
	}
}
