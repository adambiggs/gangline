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
	path := filepath.Join(directory, "codex", "sessions", "2026", "09", "24", "rollout-date-codex-id.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{\"type\":\"session_meta\",\"payload\":{\"id\":\"codex-id\"}}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	values := map[string]string{
		"GANG_STATE_ROOT": filepath.Join(directory, "state"),
		"GANG_CONFIG_DIR": filepath.Join(directory, "config"),
		"GANG_SESSION":    "resume-preflight-test",
		"GANG_TMUX":       filepath.Join(directory, "must-not-run-tmux"),
	}
	for _, collar := range []string{"claude-code", "codex"} {
		reader := &resumeAssignmentReader{}
		cmd := command{
			stdin: reader, stdout: io.Discard, stderr: io.Discard,
			getenv:      func(key string) string { return values[key] },
			lookupEnv:   func(key string) (string, bool) { value, ok := values[key]; return value, ok },
			getwd:       func() (string, error) { return directory, nil },
			userHomeDir: func() (string, error) { return directory, nil },
		}
		err := cmd.hitch([]string{"worker", "-c", collar, "--resume", "codex-id", "--stdin"})
		if collar == "claude-code" {
			var refused commandError
			if !errors.As(err, &refused) || refused.status != exitRefused || !strings.Contains(err.Error(), `cannot verify resume session "codex-id" for collar "claude-code"`) {
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
