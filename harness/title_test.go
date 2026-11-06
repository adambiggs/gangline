package harness

import (
	"strings"
	"testing"
)

// The titles follow the shapes #{pane_title} showed for Claude Code 2.1.319
// and Codex 0.160.0 in the states named; the Codex titles are verbatim. Codex
// blinks its approval marker between [ ! ] and [ . ].
func TestEmbeddedCollarsReadPaneTitles(t *testing.T) {
	tests := []struct {
		name, collar, fixture, title string
		idle, blocked                bool
	}{
		{name: "claude spinner", collar: "claude", fixture: "claude-code-2.1.278-composer.txt", title: "◐ Fix the flaky test"},
		{name: "claude later spinner frame", collar: "claude", fixture: "claude-code-2.1.278-composer.txt", title: "◓ Fix the flaky test"},
		{name: "claude braille spinner", collar: "claude", fixture: "claude-code-2.1.278-composer.txt", title: "⠂ Fix the flaky test"},
		{name: "claude not busy", collar: "claude", fixture: "claude-code-2.1.278-composer.txt", title: "✳ Fix the flaky test", idle: true},
		{name: "claude unset title", collar: "claude", fixture: "claude-code-2.1.278-composer.txt", title: "", idle: true},
		{name: "claude unmatched title", collar: "claude", fixture: "claude-code-2.1.278-composer.txt", title: "build-host.local", idle: true},
		{name: "codex spinner", collar: "codex", fixture: "codex-0.151.0-composer.txt", title: "⠸ Run delayed echo command | codex-cwd"},
		{name: "codex idle", collar: "codex", fixture: "codex-0.151.0-composer.txt", title: "Run delayed echo command | codex-cwd", idle: true},
		{name: "codex unset title", collar: "codex", fixture: "codex-0.151.0-composer.txt", title: "", idle: true},
		{name: "codex unmatched title", collar: "codex", fixture: "codex-0.151.0-composer.txt", title: "build-host.local", idle: true},
		{name: "codex untitled task spinner", collar: "codex", fixture: "codex-0.151.0-composer.txt", title: "⠏ ⠏ | cwd"},
		{name: "codex approval", collar: "codex", fixture: "codex-0.151.0-composer.txt", title: "[ ! ] Action Required | Reply with one | codex-cwd", idle: true, blocked: true},
		{name: "codex approval blink frame", collar: "codex", fixture: "codex-0.151.0-composer.txt", title: "[ . ] Action Required | Create empty probe.txt | cwd", idle: true, blocked: true},
		{name: "codex idle without a task", collar: "codex", fixture: "codex-0.151.0-composer.txt", title: "cwd", idle: true},
		// Claude Code shows ✳ while a permission dialog is open; its title
		// cannot tell blocked from idle, so its collar names no blocked title.
		{name: "claude title during a permission dialog", collar: "claude", fixture: "claude-code-2.1.278-composer.txt", title: "✳ Reply with one", idle: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			collar, err := EmbeddedCollar(test.collar)
			if err != nil {
				t.Fatal(err)
			}
			screen := fixtureScreen(t, test.fixture)
			screen.Title = test.title
			idle, err := Idle(collar, screen)
			if err != nil || idle != test.idle {
				t.Fatalf("idle = %v, error = %v; want %v", idle, err, test.idle)
			}
			blocker, blocked, err := InputBlocked(collar, screen)
			if err != nil || blocked != test.blocked {
				t.Fatalf("blocked = %v (%q), error = %v; want %v", blocked, blocker.Evidence, err, test.blocked)
			}
			if blocked && strings.Contains(blocker.Evidence, "Reply with one") {
				t.Fatalf("blocked evidence carries title text: %q", blocker.Evidence)
			}
		})
	}
}

// The screen's own prompt evidence names the prompt; the title only adds a
// reading where the screen expressions miss it.
func TestScreenBlockedEvidenceOutranksTitle(t *testing.T) {
	collar, err := EmbeddedCollar("codex")
	if err != nil {
		t.Fatal(err)
	}
	screen := fixtureScreen(t, "codex-0.151.0-command-approval.txt")
	plain, found, err := InputBlocked(collar, screen)
	if err != nil || !found {
		t.Fatalf("approval fixture: found = %v, error = %v", found, err)
	}
	screen.Title = "[ ! ] Action Required | Reply with one | codex-cwd"
	titled, found, err := InputBlocked(collar, screen)
	if err != nil || !found || titled.Evidence != plain.Evidence {
		t.Fatalf("titled evidence = %q, found = %v, error = %v; want %q", titled.Evidence, found, err, plain.Evidence)
	}
}

func TestActivityFingerprintCountsOnlyBusyTitles(t *testing.T) {
	collar, err := EmbeddedCollar("codex")
	if err != nil {
		t.Fatal(err)
	}
	screen := fixtureScreen(t, "codex-0.151.0-composer.txt")
	fingerprint := func(title string) string {
		screen.Title = title
		return ActivityFingerprint(collar, screen)
	}
	untitled := fingerprint("")
	if untitled != ScreenFingerprint(screen) {
		t.Fatal("an untitled pane's activity fingerprint differs from its screen fingerprint")
	}
	if first, second := fingerprint("⠸ Run delayed echo command | codex-cwd"), fingerprint("⠼ Run delayed echo command | codex-cwd"); first == untitled || first == second {
		t.Fatalf("spinner frames share a fingerprint: %s %s %s", untitled, first, second)
	}
	for _, title := range []string{"[ ! ] Action Required | Create empty probe.txt | cwd", "[ . ] Action Required | Create empty probe.txt | cwd", "Run delayed echo command | codex-cwd", "cwd"} {
		if fingerprint(title) != untitled {
			t.Fatalf("title %q changed the activity fingerprint", title)
		}
	}
}

func TestCollarRejectsInvalidTitleExpressions(t *testing.T) {
	for _, field := range []string{"blocked", "wedge"} {
		t.Run(field, func(t *testing.T) {
			collar, err := EmbeddedCollar("codex")
			if err != nil {
				t.Fatal(err)
			}
			params := map[string]string{}
			invocation := &collar.Primitives.Blocked
			name := "title"
			if field == "wedge" {
				invocation, name = &collar.Primitives.Wedge, "title_busy"
			}
			for key, value := range invocation.Params {
				params[key] = value
			}
			params[name] = "("
			invocation.Params = params
			if err := validateCollar(collar); err == nil || !strings.Contains(err.Error(), name) {
				t.Fatalf("validate = %v; want an error naming %s", err, name)
			}
		})
	}
}
