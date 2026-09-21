package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSettingsEnvironmentOverridesConfiguration(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "config"), []byte("GANG_SESSION=file-team\nGANG_COLLAR=codex\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	values := map[string]string{
		"GANG_CONFIG_DIR": directory,
		"GANG_SESSION":    "environment-team",
	}
	cmd := command{
		getenv: func(name string) string { return values[name] },
		lookupEnv: func(name string) (string, bool) {
			value, ok := values[name]
			return value, ok
		},
		userHomeDir: func() (string, error) { return "/workspace/example", nil },
	}
	got, err := cmd.settings()
	if err != nil {
		t.Fatal(err)
	}
	if got.Session != "environment-team" || got.Collar != "codex" {
		t.Fatalf("settings = %#v", got)
	}
	if got.Origins["GANG_SESSION"] != "environment" || got.Origins["GANG_COLLAR"] != "config" {
		t.Fatalf("origins = %#v", got.Origins)
	}
}

func TestReadConfigurationRejectsUnknownAndRepeatedKeys(t *testing.T) {
	for _, content := range []string{
		"UNKNOWN=value\n",
		"GANG_SESSION=one\nGANG_SESSION=two\n",
		"GANG_SESSION=\n",
	} {
		directory := t.TempDir()
		filename := filepath.Join(directory, "config")
		if err := os.WriteFile(filename, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := readConfiguration(filename); err == nil {
			t.Fatalf("configuration %q passed", content)
		}
	}
}
