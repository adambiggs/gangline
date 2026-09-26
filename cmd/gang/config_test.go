package main

import (
	"os"
	"path/filepath"
	"strings"
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
	if got.Origins["GANG_SESSION"] != "env" || got.Origins["GANG_COLLAR"] != "config" {
		t.Fatalf("origins = %#v", got.Origins)
	}
}

func TestConfigShowsEnvironmentOnlyOrigins(t *testing.T) {
	directory := t.TempDir()
	values := map[string]string{
		"GANG_CONFIG_DIR":  directory,
		"GANG_STATE_ROOT":  filepath.Join(directory, "state"),
		"GANG_TMUX_SOCKET": filepath.Join(directory, "tmux.sock"),
	}
	var output strings.Builder
	cmd := command{
		stdout: &output,
		getenv: func(name string) string { return values[name] },
		lookupEnv: func(name string) (string, bool) {
			value, ok := values[name]
			return value, ok
		},
		userHomeDir: func() (string, error) { return directory, nil },
	}
	if err := cmd.config(nil); err != nil {
		t.Fatal(err)
	}
	for name, value := range values {
		want := name + "=" + value + "\tenv\n"
		if !strings.Contains(output.String(), want) {
			t.Errorf("config output lacks %q: %q", want, output.String())
		}
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

func TestSettingsLoadsOperatorLaunchArgumentsByCollar(t *testing.T) {
	directory := t.TempDir()
	content := "GANG_LAUNCH_ARGS={\"codex\":[\"--dangerously-bypass-approvals-and-sandbox\"]}\n"
	if err := os.WriteFile(filepath.Join(directory, "config"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := command{
		getenv: func(string) string { return "" },
		lookupEnv: func(name string) (string, bool) {
			if name == "GANG_CONFIG_DIR" {
				return directory, true
			}
			return "", false
		},
		userHomeDir: func() (string, error) { return "/workspace/example", nil },
	}
	got, err := cmd.settings()
	if err != nil {
		t.Fatal(err)
	}
	if len(got.LaunchArgs["codex"]) != 1 || got.LaunchArgs["codex"][0] != "--dangerously-bypass-approvals-and-sandbox" {
		t.Fatalf("launch args = %#v", got.LaunchArgs)
	}
}
