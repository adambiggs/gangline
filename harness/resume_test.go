package harness

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateResumeUsesSelectedNativeStoreAndIdentity(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CODEX_HOME", "")
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	writeResumeFixture(t, filepath.Join(home, ".codex", "sessions", "2026", "09", "24", "rollout-date-codex-id.jsonl"), `{"type":"session_meta","payload":{"id":"codex-id"}}`+"\n")
	writeResumeFixture(t, filepath.Join(home, ".claude", "projects", "project", "claude-id.jsonl"), "{\"type\":\"file-history-snapshot\"}\n{\"sessionId\":\"claude-id\"}\n")
	for _, test := range []struct {
		collar, session string
		valid           bool
	}{
		{"codex", "codex-id", true}, {"claude-code", "claude-id", true},
		{"claude-code", "codex-id", false}, {"codex", "claude-id", false},
		{"codex", "missing-id", false}, {"claude-code", "missing-id", false},
		{"codex", "../codex-id", false}, {"claude-code", "*", false},
	} {
		t.Run(test.collar+"/"+test.session, func(t *testing.T) {
			c, err := EmbeddedCollar(test.collar)
			if err != nil {
				t.Fatal(err)
			}
			err = ValidateResume(c, test.session)
			if (err == nil) != test.valid {
				t.Fatalf("ValidateResume = %v; valid = %v", err, test.valid)
			}
			if err != nil && !strings.Contains(err.Error(), "cannot verify resume session") {
				t.Fatal(err)
			}
		})
	}
}

func TestValidateResumeHonorsNativeHomeAndRejectsMismatchedEvidence(t *testing.T) {
	for _, test := range []struct{ collar, key, path, good, bad string }{
		{"codex", "CODEX_HOME", "sessions/2026/09/24/rollout-date-native-id.jsonl", "{\"type\":\"session_meta\",\"payload\":{\"id\":\"native-id\"}}\n", "{\"type\":\"session_meta\",\"payload\":{\"id\":\"wrong-id\"}}\n"},
		{"claude-code", "CLAUDE_CONFIG_DIR", "projects/project/native-id.jsonl", "{\"sessionId\":\"native-id\"}\n", "{\"sessionId\":\"wrong-id\"}\n"},
	} {
		t.Run(test.collar, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "native[store]")
			t.Setenv(test.key, root)
			c, err := EmbeddedCollar(test.collar)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(root, test.path)
			writeResumeFixture(t, path, test.good)
			if err := ValidateResume(c, "native-id"); err != nil {
				t.Fatal(err)
			}
			for _, content := range []string{test.bad, "", "broken json\n"} {
				writeResumeFixture(t, path, content)
				if err := ValidateResume(c, "native-id"); err == nil {
					t.Fatalf("accepted transcript %q", content)
				}
			}
			writeResumeFixture(t, path, test.good)
			t.Setenv(test.key, t.TempDir())
			c.Launch.Env = map[string]string{test.key: root}
			if err := ValidateResume(c, "native-id"); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestValidateResumeKeepsCustomCollarWithoutTelemetry(t *testing.T) {
	c, err := EmbeddedCollar("codex")
	if err != nil {
		t.Fatal(err)
	}
	c.Primitives.Telemetry = nil
	// ResumeArgs are sufficient for a custom collar; telemetry is optional.
	if err := ValidateResume(c, "native-id"); err != nil {
		t.Fatalf("ValidateResume = %v", err)
	}
}

func writeResumeFixture(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}
