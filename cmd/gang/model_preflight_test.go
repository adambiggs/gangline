package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const fakeCodexCatalog = `{"models":[{"slug":"gpt-one","supported_reasoning_levels":[{"effort":"low"},{"effort":"high"}]}]}`

// hitchModelPreflight runs hitch against a fake native codex whose catalog
// prints catalog and exits with status. The assignment reader is the barrier
// that proves preflight let the launch continue.
func hitchModelPreflight(t *testing.T, catalog string, status int, args ...string) (*resumeAssignmentReader, string, error) {
	t.Helper()
	directory := t.TempDir()
	bin := filepath.Join(directory, "bin")
	if err := os.MkdirAll(bin, 0700); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\nprintf '%s\\n' '" + catalog + "'\nprintf 'catalog credential failure\\n' >&2\nexit " + strconv.Itoa(status) + "\n"
	if err := os.WriteFile(filepath.Join(bin, "codex"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	values := map[string]string{
		"GANG_STATE_ROOT": filepath.Join(directory, "state"),
		"GANG_CONFIG_DIR": filepath.Join(directory, "config"),
		"GANG_SESSION":    "model-preflight-test",
		"GANG_TMUX":       filepath.Join(directory, "must-not-run-tmux"),
	}
	reader := &resumeAssignmentReader{}
	var stderr bytes.Buffer
	cmd := command{
		stdin: reader, stdout: &bytes.Buffer{}, stderr: &stderr,
		getenv:      func(key string) string { return values[key] },
		lookupEnv:   func(key string) (string, bool) { value, ok := values[key]; return value, ok },
		getwd:       func() (string, error) { return directory, nil },
		userHomeDir: func() (string, error) { return directory, nil },
	}
	err := cmd.hitch(append([]string{"worker", "-c", "codex", "--stdin"}, args...))
	if _, statErr := os.Stat(values["GANG_STATE_ROOT"]); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("preflight created state: %v", statErr)
	}
	return reader, stderr.String(), err
}

func passedPreflight(reader *resumeAssignmentReader, err error) bool {
	return reader.read && err != nil && strings.Contains(err.Error(), "assignment read barrier")
}

func TestHitchLeavesUnlistedModelToNativeCLI(t *testing.T) {
	reader, _, err := hitchModelPreflight(t, fakeCodexCatalog, 0, "-m", "gpt-unlisted")
	if !passedPreflight(reader, err) {
		t.Fatalf("unlisted model did not pass preflight: %v", err)
	}
}

func TestHitchLaunchesWhenModelCatalogFailsAndShowsItsDiagnostic(t *testing.T) {
	for _, args := range [][]string{{"-m", "gpt-one"}, {"-m", "gpt-one", "-e", "high"}} {
		reader, stderr, err := hitchModelPreflight(t, "", 9, args...)
		if !passedPreflight(reader, err) {
			t.Fatalf("%v: catalog failure vetoed launch: %v", args, err)
		}
		// Without --effort the catalog is not consulted at all.
		if shown := strings.Contains(stderr, "catalog credential failure"); shown != (len(args) > 2) {
			t.Fatalf("%v: catalog diagnostic shown = %v; stderr = %q", args, shown, stderr)
		}
	}
}

func TestHitchRefusesEffortTheCatalogDoesNotListForTheModel(t *testing.T) {
	reader, _, err := hitchModelPreflight(t, fakeCodexCatalog, 0, "-m", "gpt-one", "-e", "xhigh")
	var refused commandError
	if !errors.As(err, &refused) || refused.status != exitRefused || !strings.Contains(err.Error(), `"xhigh"`) || !strings.Contains(err.Error(), "low, high") {
		t.Fatalf("unlisted effort error = %v", err)
	}
	if reader.read {
		t.Fatal("refused effort consumed assignment")
	}
}
