package main

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
)

func helpOptionCounts(output string) map[string]int {
	counts := map[string]int{}
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		for _, field := range fields {
			name := strings.TrimSuffix(field, ",")
			if !strings.HasPrefix(name, "-") {
				break
			}
			counts[name]++
		}
	}
	return counts
}

func helpOptionNames(output string) map[string]bool {
	names := map[string]bool{}
	for name := range helpOptionCounts(output) {
		names[name] = true
	}
	return names
}

func TestHelpGroupsAliases(t *testing.T) {
	aliases := map[string][]string{
		"up":     {"-c, --collar COLLAR", "-d, --dir DIR", "-m, --model MODEL", "-e, --effort EFFORT", "-t, --task TASK", "-r, --role ROLE"},
		"hitch":  {"-c, --collar COLLAR", "-d, --dir DIR", "-m, --model MODEL", "-e, --effort EFFORT", "-t, --task TASK", "-r, --role ROLE"},
		"adopt":  {"-c, --collar COLLAR"},
		"models": {"-c, --collar COLLAR"},
	}
	for name := range commandUsage {
		var stdout, stderr bytes.Buffer
		if status := run([]string{"help", name}, strings.NewReader(""), &stdout, &stderr); status != exitOK {
			t.Fatalf("help %s: status=%d stderr=%q", name, status, stderr.String())
		}
		if strings.Count(stdout.String(), "  -h, --help") != 1 {
			t.Errorf("help %s does not group -h and --help once", name)
		}
		for _, label := range aliases[name] {
			if strings.Count(stdout.String(), "  "+label) != 1 {
				t.Errorf("help %s does not show %q once", name, label)
			}
		}
	}
	var stdout, stderr bytes.Buffer
	if status := run([]string{"--help"}, strings.NewReader(""), &stdout, &stderr); status != exitOK {
		t.Fatalf("top-level help: status=%d stderr=%q", status, stderr.String())
	}
	if strings.Count(stdout.String(), "  -h, --help") != 1 {
		t.Error("top-level help does not group -h and --help once")
	}
}

func expectedHelpOptions(name string) map[string]bool {
	want := map[string]bool{"--help": true, "-h": true}
	for _, option := range optionsFor(name) {
		want[optionSpelling(option.name)] = true
	}
	return want
}

func helpHasOptionRow(output string, option optionSpec) bool {
	for _, line := range strings.Split(output, "\n") {
		if helpOptionNames(line)[optionSpelling(option.name)] &&
			strings.Contains(line, optionArgument(option)) &&
			strings.Contains(line, option.meaning) {
			return true
		}
	}
	return false
}

func TestCommandHelpMatchesAcceptedFlags(t *testing.T) {
	for name := range commandUsage {
		t.Run(name, func(t *testing.T) {
			var help, direct, stderr bytes.Buffer
			if status := run([]string{"help", name}, strings.NewReader(""), &help, &stderr); status != exitOK {
				t.Fatalf("help %s: status=%d stderr=%q", name, status, stderr.String())
			}
			if status := run([]string{name, "--help"}, strings.NewReader(""), &direct, &stderr); status != exitOK {
				t.Fatalf("%s --help: status=%d stderr=%q", name, status, stderr.String())
			}
			if help.String() != direct.String() {
				t.Fatal("help paths differ")
			}
			if name == "hitch" {
				if !strings.Contains(help.String(), "\n\nOptions:\n") || !strings.Contains(help.String(), "\n\nHelp:\n") || !strings.HasSuffix(help.String(), "\n\n"+flagSyntaxHelp) {
					t.Fatal("hitch help sections are not separated or have the wrong flag syntax")
				}
				for _, line := range strings.Split(help.String(), "\n") {
					if len(line) >= 79 {
						t.Errorf("hitch help line has %d columns: %q", len(line), line)
					}
				}
			}
			got := helpOptionNames(help.String())
			want := expectedHelpOptions(name)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("help flags = %v, parser flags = %v", got, want)
			}
			for flag, count := range helpOptionCounts(help.String()) {
				if count != 1 {
					t.Errorf("help prints %s %d times", flag, count)
				}
			}
			for _, option := range optionsFor(name) {
				if !helpHasOptionRow(help.String(), option) {
					t.Fatalf("missing flag shape or meaning for %s", optionSpelling(option.name))
				}
			}
		})
	}
}

func TestTopLevelHelpShowsGlobalFlags(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if status := run([]string{"--help"}, strings.NewReader(""), &stdout, &stderr); status != exitOK {
		t.Fatalf("status=%d stderr=%q", status, stderr.String())
	}
	output := stdout.String()
	if got, want := helpOptionCounts(output), map[string]int{"-h": 1, "--help": 1, "--version": 1}; !reflect.DeepEqual(got, want) {
		t.Errorf("top-level flags = %v, want %v", got, want)
	}
	if strings.Contains(output, "Command flags") || strings.Contains(output, "Flag syntax:") {
		t.Error("top-level help contains the old flag sections")
	}
	if !strings.HasPrefix(output, "usage: gang <command> [arguments]\n\nStart and end a team:\n") ||
		!strings.Contains(output, "\n\nInstallation:\n") ||
		!strings.Contains(output, "\n\nGlobal flags:\n") ||
		!strings.HasSuffix(output, "  "+flagSyntaxHelp) {
		t.Error("top-level help sections are not separated or have the wrong flag syntax")
	}
	for _, line := range strings.Split(output, "\n") {
		if len(line) >= 79 {
			t.Errorf("top-level help line has %d columns: %q", len(line), line)
		}
	}
}

func TestMissingOrExtraHelpFlagIsDetected(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if status := run([]string{"help", "send"}, strings.NewReader(""), &stdout, &stderr); status != exitOK {
		t.Fatalf("status=%d stderr=%q", status, stderr.String())
	}
	correct := helpOptionNames(stdout.String())
	if !reflect.DeepEqual(correct, expectedHelpOptions("send")) {
		t.Fatal("send help is already inconsistent")
	}
	missing := strings.Replace(stdout.String(), "  --live-only", "  live-only", 1)
	if reflect.DeepEqual(helpOptionNames(missing), expectedHelpOptions("send")) {
		t.Fatal("a missing parsed flag escaped detection")
	}
	extra := stdout.String() + "  --invented             undocumented parser option\n"
	if reflect.DeepEqual(helpOptionNames(extra), expectedHelpOptions("send")) {
		t.Fatal("an unaccepted help flag escaped detection")
	}
}

func TestHelpAfterOtherArguments(t *testing.T) {
	for name := range commandUsage {
		for _, helpFlag := range []string{"--help", "-h"} {
			var stdout, stderr bytes.Buffer
			arguments := []string{name, "unused", helpFlag}
			if status := run(arguments, strings.NewReader(""), &stdout, &stderr); status != exitOK {
				t.Errorf("run(%q) status=%d stderr=%q", arguments, status, stderr.String())
			} else if !strings.Contains(stdout.String(), "usage: gang "+name) {
				t.Errorf("run(%q) lacks command help", arguments)
			}
		}
	}
}

func TestEveryCommandFlagParserConstructs(t *testing.T) {
	cmd := command{stdin: strings.NewReader("")}
	bad := []string{"--unlisted"}
	checks := map[string]func() error{
		"hitch":      func() error { _, err := parseHitch([]string{"worker", "--unlisted"}, "codex", "/work"); return err },
		"adopt":      func() error { _, err := parseCollarFlags("adopt", bad, "codex"); return err },
		"send":       func() error { _, err := parseSend([]string{"worker", "--unlisted"}); return err },
		"interrupt":  func() error { return cmd.interrupt(bad) },
		"compact":    func() error { _, err := parseCompact(bad); return err },
		"statusline": func() error { return cmd.statusline(bad) },
		"context":    func() error { return cmd.context(bad) },
		"log":        func() error { _, _, err := parseLogFilter(bad, true); return err },
		"limits":     func() error { return cmd.limits(bad) },
		"wait":       func() error { _, err := parseWait([]string{"worker", "--unlisted"}); return err },
		"status":     func() error { return cmd.status(bad) },
		"tick":       func() error { return cmd.tick(bad) },
		"capture":    func() error { return cmd.capture(bad) },
		"roster":     func() error { return cmd.roster(bad) },
		"models":     func() error { _, err := parseCollarFlags("models", bad, "codex"); return err },
		"upgrade":    func() error { return cmd.upgrade(bad) },
	}
	for name, check := range checks {
		t.Run(name, func(t *testing.T) {
			if err := check(); err == nil {
				t.Fatal("unlisted option was accepted")
			}
		})
	}
	for name := range commandOptions {
		if _, ok := checks[name]; !ok {
			t.Errorf("flag parser for %s has no construction check", name)
		}
	}
}

func TestUpRecoverUsesHitchRecoveryArguments(t *testing.T) {
	for _, recoverFlag := range []string{"--recover", "-recover", "--recover=true"} {
		arguments := upHitchArguments("lead", []string{recoverFlag})
		options, err := parseHitch(arguments, "codex", "/work")
		if err != nil || !options.Recover {
			t.Errorf("%s: arguments=%v options=%+v err=%v", recoverFlag, arguments, options, err)
		}
	}
}
