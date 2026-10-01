package main

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type resumeAssignmentReader struct{ read bool }

func (r *resumeAssignmentReader) Read([]byte) (int, error) {
	r.read = true
	return 0, errors.New("assignment read barrier")
}

func TestHitchRejectsForeignResumeBeforeReadingAssignmentOrClaimingAgent(t *testing.T) {
	directory := t.TempDir()
	t.Setenv("CODEX_HOME", filepath.Join(directory, "codex"))
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(directory, "claude"))
	for path, content := range map[string]string{
		filepath.Join(directory, "codex", "sessions", "2026", "09", "24", "rollout-date-codex-id.jsonl"): "{\"type\":\"session_meta\",\"payload\":{\"id\":\"codex-id\"}}\n",
		filepath.Join(directory, "claude", "projects", "project", "codex-id.jsonl"):                      "{\"sessionId\":\"claude-id\"}\n",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	values := map[string]string{
		"GANG_STATE_ROOT": filepath.Join(directory, "state"),
		"GANG_CONFIG_DIR": filepath.Join(directory, "config"),
		"GANG_SESSION":    "resume-preflight-test",
		"GANG_TMUX":       filepath.Join(directory, "must-not-run-tmux"),
	}
	for _, collar := range []string{"claude", "codex"} {
		reader := &resumeAssignmentReader{}
		cmd := command{
			stdin: reader, stdout: io.Discard, stderr: io.Discard,
			getenv:      func(key string) string { return values[key] },
			lookupEnv:   func(key string) (string, bool) { value, ok := values[key]; return value, ok },
			getwd:       func() (string, error) { return directory, nil },
			userHomeDir: func() (string, error) { return directory, nil },
		}
		err := cmd.hitch([]string{"worker", "-c", collar, "--resume", "codex-id", "--stdin"})
		if collar == "claude" {
			var refused commandError
			if !errors.As(err, &refused) || refused.status != exitRefused || !strings.Contains(err.Error(), `resume session "codex-id" for collar "claude"`) || !strings.Contains(err.Error(), `"claude-id"`) {
				t.Errorf("foreign resume error = %v", err)
			}
			if reader.read {
				t.Error("foreign resume consumed assignment")
			}
		} else if !reader.read || err == nil || !strings.Contains(err.Error(), "assignment read barrier") {
			t.Errorf("valid resume did not pass preflight: %v", err)
		}
		if _, err := os.Stat(values["GANG_STATE_ROOT"]); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("preflight created state: %v", err)
		}
	}
}

func TestHitchLeavesResumeWithoutTranscriptToNativeCLI(t *testing.T) {
	directory := t.TempDir()
	t.Setenv("CODEX_HOME", filepath.Join(directory, "codex"))
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(directory, "claude"))
	values := map[string]string{
		"GANG_STATE_ROOT": filepath.Join(directory, "state"),
		"GANG_CONFIG_DIR": filepath.Join(directory, "config"),
		"GANG_SESSION":    "resume-preflight-test",
		"GANG_TMUX":       filepath.Join(directory, "must-not-run-tmux"),
	}
	for _, collar := range []string{"claude", "codex"} {
		reader := &resumeAssignmentReader{}
		var stderr strings.Builder
		cmd := command{
			stdin: reader, stdout: io.Discard, stderr: &stderr,
			getenv:      func(key string) string { return values[key] },
			lookupEnv:   func(key string) (string, bool) { value, ok := values[key]; return value, ok },
			getwd:       func() (string, error) { return directory, nil },
			userHomeDir: func() (string, error) { return directory, nil },
		}
		err := cmd.hitch([]string{"worker", "-c", collar, "--resume", "unlisted-id", "--stdin"})
		if !reader.read || err == nil || !strings.Contains(err.Error(), "assignment read barrier") {
			t.Errorf("%s: resume without transcript did not pass preflight: %v", collar, err)
		}
		if !strings.Contains(stderr.String(), `resume session "unlisted-id" left to the native CLI`) {
			t.Errorf("%s: unchecked resume was silent; stderr = %q", collar, stderr.String())
		}
	}
}
