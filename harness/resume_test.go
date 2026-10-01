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
		collar, session  string
		valid, unchecked bool
	}{
		{"codex", "codex-id", true, false}, {"claude", "claude-id", true, false},
		// Without a transcript to read, the native CLI decides.
		{"claude", "codex-id", true, true}, {"codex", "claude-id", true, true},
		{"codex", "missing-id", true, true}, {"claude", "missing-id", true, true},
		{"codex", "../codex-id", false, false}, {"claude", "*", false, false},
	} {
		t.Run(test.collar+"/"+test.session, func(t *testing.T) {
			c, err := EmbeddedCollar(test.collar)
			if err != nil {
				t.Fatal(err)
			}
			unverified, err := ValidateResume(c, test.session)
			if (err == nil) != test.valid || (unverified != "") != test.unchecked {
				t.Fatalf("ValidateResume = %q, %v; valid = %v, unchecked = %v", unverified, err, test.valid, test.unchecked)
			}
		})
	}
	t.Setenv("HOME", "")
	c, err := EmbeddedCollar("codex")
	if err != nil {
		t.Fatal(err)
	}
	if unverified, err := ValidateResume(c, "codex-id"); err != nil || unverified == "" {
		t.Fatalf("unknown native store = %q, %v", unverified, err)
	}
}

func TestValidateResumeHonorsNativeHomeAndRejectsOnlyAForeignIdentity(t *testing.T) {
	for _, test := range []struct{ collar, key, path, good, bad string }{
		{"codex", "CODEX_HOME", "sessions/2026/09/24/rollout-date-native-id.jsonl", "{\"type\":\"session_meta\",\"payload\":{\"id\":\"native-id\"}}\n", "{\"type\":\"session_meta\",\"payload\":{\"id\":\"wrong-id\"}}\n"},
		{"claude", "CLAUDE_CONFIG_DIR", "projects/project/native-id.jsonl", "{\"sessionId\":\"native-id\"}\n", "{\"sessionId\":\"wrong-id\"}\n"},
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
			if unverified, err := ValidateResume(c, "native-id"); err != nil || unverified != "" {
				t.Fatalf("own transcript = %q, %v", unverified, err)
			}
			writeResumeFixture(t, path, test.bad)
			if _, err := ValidateResume(c, "native-id"); err == nil || !strings.Contains(err.Error(), `"wrong-id"`) {
				t.Fatalf("foreign transcript = %v", err)
			}
			for _, content := range []string{"", "broken json\n"} {
				writeResumeFixture(t, path, content)
				if unverified, err := ValidateResume(c, "native-id"); err != nil || unverified == "" {
					t.Fatalf("unreadable transcript %q = %q, %v", content, unverified, err)
				}
			}
			// A second transcript under the same ID: one readable match
			// settles it, one unreadable leaves it unknown.
			other := strings.Replace(path, "/project/", "/other/", 1)
			if test.collar == "codex" {
				other = strings.Replace(path, "/24/", "/25/", 1)
			}
			writeResumeFixture(t, path, test.bad)
			writeResumeFixture(t, other, test.good)
			if unverified, err := ValidateResume(c, "native-id"); err != nil || unverified != "" {
				t.Fatalf("one matching transcript = %q, %v", unverified, err)
			}
			writeResumeFixture(t, other, "broken json\n")
			if unverified, err := ValidateResume(c, "native-id"); err != nil || unverified == "" {
				t.Fatalf("foreign beside unreadable transcript = %q, %v", unverified, err)
			}
			if err := os.Remove(other); err != nil {
				t.Fatal(err)
			}
			writeResumeFixture(t, path, test.good)
			t.Setenv(test.key, t.TempDir())
			c.Launch.Env = map[string]string{test.key: root}
			if _, err := ValidateResume(c, "native-id"); err != nil {
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
	if _, err := ValidateResume(c, "native-id"); err != nil {
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
