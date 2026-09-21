package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestVersionAndHelpDoNotLoadRuntimeConfiguration(t *testing.T) {
	for _, test := range []struct {
		args []string
		want string
	}{
		{args: nil, want: "gang — a team"},
		{args: []string{"help"}, want: "usage: gang <command>"},
		{args: []string{"hitch", "--help"}, want: "usage: gang hitch NAME"},
		{args: []string{"--version"}, want: "gangline dev"},
	} {
		var stdout, stderr bytes.Buffer
		status := runWithInput(test.args, strings.NewReader(""), &stdout, &stderr)
		if status != exitOK {
			t.Fatalf("run(%q) status = %d, stderr = %q", test.args, status, stderr.String())
		}
		if !strings.Contains(stdout.String(), test.want) {
			t.Fatalf("run(%q) stdout = %q, want substring %q", test.args, stdout.String(), test.want)
		}
	}
}

func TestArgumentErrorsAreUsageErrors(t *testing.T) {
	var stdout, stderr bytes.Buffer
	status := runWithInput([]string{"unknown"}, strings.NewReader(""), &stdout, &stderr)
	if status != exitUsage {
		t.Fatalf("status = %d, want %d", status, exitUsage)
	}
	if !strings.Contains(stderr.String(), "unknown command") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestSettingsUseXDGStateRoot(t *testing.T) {
	cmd := command{
		getenv: func(name string) string {
			values := map[string]string{
				"XDG_STATE_HOME":  "/state",
				"XDG_CONFIG_HOME": "/config",
			}
			return values[name]
		},
		userHomeDir: func() (string, error) { return "/workspace/example", nil },
	}
	got, err := cmd.settings()
	if err != nil {
		t.Fatal(err)
	}
	if got.StateRoot != "/state/gangline" || got.ConfigDir != "/config/gangline" {
		t.Fatalf("settings = %#v", got)
	}
}
