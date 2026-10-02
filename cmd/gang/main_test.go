package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestVersionAndHelpDoNotLoadRuntimeConfiguration(t *testing.T) {
	t.Setenv("GANG_CAPACITY_TIMEOUT", "invalid-duration")
	for _, test := range []struct {
		args []string
		want string
	}{
		{args: nil, want: "gang — a team"},
		{args: []string{"help"}, want: "usage: gang <command>"},
		{args: []string{"hitch", "--help"}, want: "usage: gang hitch NAME"},
		{args: []string{"--version"}, want: "gangline dev"},
		{args: []string{"version"}, want: "gangline dev"},
		{args: []string{"version", "--help"}, want: "usage: gang version"},
		{args: []string{"--version", "--help"}, want: "usage: gang version"},
		{args: []string{"--help", "--version"}, want: "usage: gang version"},
		{args: []string{"-h", "--version"}, want: "usage: gang version"},
		{args: []string{"help", "--version"}, want: "usage: gang version"},
		{args: []string{"help", "--", "--version"}, want: "usage: gang version"},
	} {
		var stdout, stderr bytes.Buffer
		status := run(test.args, strings.NewReader(""), &stdout, &stderr)
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
	status := run([]string{"unknown"}, strings.NewReader(""), &stdout, &stderr)
	if status != exitUsage {
		t.Fatalf("status = %d, want %d", status, exitUsage)
	}
	if !strings.Contains(stderr.String(), "unknown command") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestHelpForUnknownCommandIsUsageError(t *testing.T) {
	var out, errOut bytes.Buffer
	if got := run([]string{"help", "bogus", "--help"}, strings.NewReader(""), &out, &errOut); got != exitUsage || !strings.Contains(errOut.String(), "unknown command \"bogus\"") {
		t.Fatalf("status=%d stderr=%q", got, errOut.String())
	}
}

func TestRemovedCommandsAreUnknown(t *testing.T) {
	for _, name := range []string{"cap", "flush", "idle", "run", "usage"} {
		var stdout, stderr bytes.Buffer
		status := run([]string{name}, strings.NewReader(""), &stdout, &stderr)
		if status != exitUsage {
			t.Fatalf("run(%q) status = %d, want %d", name, status, exitUsage)
		}
		if !strings.Contains(stderr.String(), "unknown command") {
			t.Fatalf("run(%q) stderr = %q, want unknown command", name, stderr.String())
		}
	}
}

func TestCommandsRejectIgnoredArguments(t *testing.T) {
	for _, arguments := range [][]string{
		{"wait", "worker", "extra"},
		{"limits", "--history"},
	} {
		var stdout, stderr bytes.Buffer
		status := run(arguments, strings.NewReader(""), &stdout, &stderr)
		if status != exitUsage {
			t.Fatalf("run(%q) status = %d, want %d", arguments, status, exitUsage)
		}
	}
}

func TestVersionSpellingsPrintTheSameUsage(t *testing.T) {
	for _, args := range [][]string{{"version", "extra"}, {"--version", "extra"}} {
		var stdout, stderr bytes.Buffer
		status := run(args, strings.NewReader(""), &stdout, &stderr)
		want := "gang: version: unexpected argument \"extra\"\nusage: gang version\n"
		if status != exitUsage || stderr.String() != want {
			t.Errorf("run(%q) status=%d stderr=%q; want %q", args, status, stderr.String(), want)
		}
	}
}

func TestOptionErrorsStateTheAcceptedForm(t *testing.T) {
	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"roster", "--wat"}, "roster: unknown option --wat\n"},
		{[]string{"status", "--wat"}, "status: unknown option --wat\n"},
		{[]string{"interrupt", "--wat"}, "interrupt: unknown option --wat\n"},
		{[]string{"upgrade", "--wat"}, "upgrade: unknown option --wat\n"},
		{[]string{"up", "--bogus"}, "up: unknown option --bogus\n"},
		{[]string{"models", "--bogus"}, "models: unknown option --bogus\n"},
		{[]string{"interrupt", "-m"}, "interrupt: -m needs a value (expected REASON)"},
		{[]string{"wait", "worker", "--timeout"}, "wait: --timeout needs a value (expected DURATION)"},
		{[]string{"wait", "worker", "--timeout=abc"}, `wait: invalid --timeout "abc" (expected a duration such as 30s or 5m)`},
		{[]string{"roster", "--json=maybe"}, `roster: invalid --json "maybe" (expected true or false)`},
		{[]string{"send", "worker", "--from", "bad name", "hi"}, `send: invalid --from "bad name" (expected letters, digits, '.', '_' or '-', starting with a letter or digit; hitch and gangline are reserved)`},
		{[]string{"send", "worker", "---from", "x"}, "send: malformed option ---from (expected one or two dashes before the name)"},
		{[]string{"status", "--team"}, "status: --team needs a value (expected TEAM)"},
		{[]string{"collar", "check", "--bogus"}, `invalid collar name "--bogus" (expected lowercase letters, digits or '-', starting with a letter)`},
		{[]string{"help", "bogus"}, `help: unknown command "bogus" (run 'gang help')`},
		{[]string{"roster", "extra"}, "unexpected argument \"extra\""},
		{[]string{"version", "x"}, `gang: version: unexpected argument "x"`},
		{[]string{"--version", "x"}, `gang: version: unexpected argument "x"`},
		{[]string{"collars", "x"}, `gang: collars: unexpected argument "x"`},
		{[]string{"roles", "x"}, `gang: roles: unexpected argument "x"`},
		{[]string{"config", "x"}, `gang: config: unexpected argument "x"`},
	} {
		var stdout, stderr bytes.Buffer
		status := run(test.args, strings.NewReader(""), &stdout, &stderr)
		if status != exitUsage || !strings.Contains(stderr.String(), test.want) {
			t.Errorf("run(%q) status=%d stderr=%q; want %q", test.args, status, stderr.String(), test.want)
		}
		for _, leaked := range flagPackageText {
			if strings.Contains(stderr.String(), leaked) {
				t.Errorf("run(%q) stderr=%q carries flag-package text %q", test.args, stderr.String(), leaked)
			}
		}
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

func TestHookHelpAndModelsIndex(t *testing.T) {
	for _, args := range [][]string{{"hook", "--help"}, {"help", "hook"}} {
		var stdout, stderr bytes.Buffer
		if status := run(args, strings.NewReader(""), &stdout, &stderr); status != exitOK {
			t.Fatalf("run(%q) status=%d stderr=%q", args, status, stderr.String())
		}
		for _, want := range []string{"usage: gang hook", "stdin", "GANGLINE_HITCH_ID"} {
			if !strings.Contains(stdout.String(), want) {
				t.Fatalf("run(%q) output %q lacks %q", args, stdout.String(), want)
			}
		}
	}
	var stdout, stderr bytes.Buffer
	if status := run([]string{"help"}, strings.NewReader(""), &stdout, &stderr); status != exitOK {
		t.Fatalf("help status=%d stderr=%q", status, stderr.String())
	}
	if !strings.Contains(stdout.String(), "models    list models for a collar (-c)") {
		t.Fatalf("models index entry: %q", stdout.String())
	}
}
