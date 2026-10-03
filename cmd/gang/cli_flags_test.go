package main

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"
)

func TestEveryShortOptionHasLongAlias(t *testing.T) {
	for name, options := range commandOptions {
		for i, option := range options {
			if len(option.name) != 1 {
				continue
			}
			if i+1 == len(options) || len(options[i+1].name) == 1 || options[i+1].argument != option.argument || options[i+1].meaning != option.meaning {
				t.Errorf("%s -%s has no adjacent long alias with the same argument and meaning", name, option.name)
			}
		}
	}
	f := newStateFixture(t)
	if err := f.cmd.limits([]string{"--collar", "codex", "worker"}); err == nil || !strings.Contains(err.Error(), "no agent") {
		t.Errorf("limits --collar = %v", err)
	}
	if err := f.cmd.interrupt([]string{"--message", "stop", "a", "b"}); err == nil || !strings.Contains(err.Error(), `unexpected argument "b"`) {
		t.Errorf("interrupt --message = %v", err)
	}
}

func TestCaptureTakesLinesOption(t *testing.T) {
	f := newStateFixture(t)
	f.env["TMUX_PANE"] = "%1"
	for _, args := range [][]string{{"-n", "1"}, {"--lines", "1"}, {"--lines=1"}, {"-n1"}} {
		f.out.Reset()
		if err := f.cmd.capture(args); err != nil || f.out.Len() == 0 {
			t.Errorf("capture %q = %v, output %q", args, err, f.out.String())
		}
	}
	for _, args := range [][]string{{"worker", "40"}, {"-n", "0"}, {"--lines", "x"}, {"--composer", "-n", "5"}} {
		var usage commandError
		if err := f.cmd.capture(args); !errors.As(err, &usage) || usage.status != exitUsage {
			t.Errorf("capture %q = %v, want usage error", args, err)
		}
	}
}

func TestTickAgentResolvesRegisteredName(t *testing.T) {
	f := newStateFixture(t)
	f.add(t, "a", "worker", "missing-collar")
	if err := f.cmd.tick([]string{"--agent", "worker"}); err == nil {
		t.Fatal("tick succeeded with an unavailable collar")
	}
	failures := tickFailures(t, f)
	if len(failures) != 1 || failures[0].HitchID != "a" || !strings.Contains(failures[0].Reason, `"missing-collar"`) {
		t.Fatalf("failures = %+v, want the collar failure of hitch a", failures)
	}
}

func TestHitchAttachedShortValuesAndRecoverTerminator(t *testing.T) {
	got, err := parseHitch([]string{"worker", "-ccodex", "-mgpt", "-d/work"}, "claude", "/default")
	if err != nil || got.Collar != "codex" || got.Model != "gpt" || got.Directory != "/work" {
		t.Errorf("attached short values = %+v, %v", got, err)
	}
	for _, args := range [][]string{{"worker", "--recover", "--"}, {"--recover", "--", "worker"}, {"--recover", "worker"}} {
		if got, err := parseHitch(args, "claude", "/default"); err != nil || !got.Recover || got.Name != "worker" {
			t.Errorf("hitch %q = %+v, %v", args, got, err)
		}
	}
	for _, args := range [][]string{{"worker", "--recover", "-c", "codex"}, {"worker", "--recover", "--", "extra"}, {"worker", "--recover", "--stdin"}} {
		if _, err := parseHitch(args, "claude", "/default"); err == nil {
			t.Errorf("hitch %q accepted", args)
		}
	}
	if _, err := parseHitch([]string{"worker", "-yes"}, "claude", "/default"); err == nil || !strings.Contains(err.Error(), "-yes") {
		t.Errorf("undefined attached form = %v", err)
	}
	if !strings.Contains(flagSyntaxHelp, "-cVALUE") || !strings.Contains(flagSyntaxHelp, "--") {
		t.Errorf("flag syntax help = %q", flagSyntaxHelp)
	}
}

func TestOneClearSpelling(t *testing.T) {
	for name, options := range commandOptions {
		for _, option := range options {
			for _, literal := range []string{"clear", "off"} {
				for _, value := range strings.Split(option.argument, "|") {
					if value == literal {
						t.Errorf("%s --%s takes the literal %q", name, option.name, literal)
					}
				}
			}
		}
	}
	for _, name := range []string{"send", "context", "snooze"} {
		found := false
		for _, option := range commandOptions[name] {
			found = found || option.name == "clear" && option.argument == ""
		}
		if !found {
			t.Errorf("%s has no --clear switch", name)
		}
	}
	if got, err := parseSend([]string{"worker", "--clear"}); err != nil || !got.Clear {
		t.Errorf("send --clear = %+v, %v", got, err)
	}
	for _, args := range [][]string{{"worker", "--clear", "body"}, {"worker", "--clear", "--at", "1h"}, {"worker", "--clear", "--live-only"}} {
		if _, err := parseSend(args); err == nil {
			t.Errorf("send %q accepted", args)
		}
	}
	f := newStateFixture(t)
	for _, args := range [][]string{{"--clear", "worker"}, {"--clear", "--widget", "worker"}} {
		var usage commandError
		if err := f.cmd.context(args); !errors.As(err, &usage) || usage.status != exitUsage {
			t.Errorf("context %q = %v, want usage error", args, err)
		}
	}
}

func TestHelpAnywhereBeforeTerminator(t *testing.T) {
	for _, args := range [][]string{
		{"hitch", "worker", "--help"},
		{"send", "worker", "body", "-h"},
		{"hitch", "--collar", "codex", "--help"},
		{"hitch", "worker", "-ccodex", "-h"},
		{"rename", "old", "new", "--help"},
	} {
		if !helpRequested(args[0], args[1:]) {
			t.Errorf("helpRequested(%q) = false", args)
		}
	}
	for _, args := range [][]string{
		{"send", "worker", "--", "-h"},
		{"rename", "old", "--", "--help"},
		{"interrupt", "-m", "--help"},
		{"interrupt", "--message=--help"},
		{"hitch", "worker", "-t", "-h"},
		{"log", "--agent", "--help"},
	} {
		if helpRequested(args[0], args[1:]) {
			t.Errorf("helpRequested(%q) = true", args)
		}
	}
	t.Setenv("GANG_SESSION", "../not-a-session")
	var out, errOut bytes.Buffer
	if got := run([]string{"hitch", "worker", "--help"}, strings.NewReader(""), &out, &errOut); got != exitOK || !strings.HasPrefix(out.String(), "usage: gang hitch") || errOut.Len() != 0 {
		t.Fatalf("status=%d stdout=%q stderr=%q", got, out.String(), errOut.String())
	}
}

func TestUpAttachesOnlyToTerminal(t *testing.T) {
	null, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer null.Close()
	if (command{stdin: null}).upAttaches() {
		t.Error("up attaches with stdin on the null device")
	}
	if !(command{stdin: null, terminalInput: func() bool { return true }}).upAttaches() {
		t.Error("up does not attach with terminal input")
	}
}

func TestTickAgentRefusesUnknownOutsideHooks(t *testing.T) {
	f := newStateFixture(t)
	f.add(t, "a", "worker", "codex")
	var refusal commandError
	if err := f.cmd.tick([]string{"--agent", "typo"}); !errors.As(err, &refusal) || refusal.status != exitRefused {
		t.Fatalf("tick --agent typo = %v, want refusal", err)
	}
	f.env["GANGLINE_BOUNDARY"] = `{"kind":"turn-finished"}`
	if err := f.cmd.tick([]string{"--agent", "dropped"}); err != nil {
		t.Fatalf("hook tick for a dropped hitch = %v", err)
	}
}

func TestSendRejectsBadScheduleBeforeReadingBody(t *testing.T) {
	f := newStateFixture(t)
	f.add(t, "a", "worker", "codex")
	f.env["GANG_FROM"] = ""
	f.cmd.stdin = strings.NewReader("")
	var usage commandError
	err := f.cmd.send([]string{"worker", "--from", "op", "--at", "clear"})
	if !errors.As(err, &usage) || usage.status != exitUsage || !strings.Contains(err.Error(), "--at") {
		t.Fatalf("send --at clear = %v, want an --at usage error", err)
	}
}

func TestHelpSpellingsAndMistypedLongFlags(t *testing.T) {
	for _, args := range [][]string{{"worker", "-help"}, {"worker", "--help=true"}, {"worker", "-h=true"}} {
		if !helpRequested("hitch", args) {
			t.Errorf("helpRequested(hitch %q) = false", args)
		}
	}
	for _, flag := range []string{"-modl=opus", "-rol=lead"} {
		if _, err := parseHitch([]string{"worker", flag}, "codex", "/work"); err == nil || !strings.Contains(err.Error(), flag[1:strings.Index(flag, "=")]) {
			t.Errorf("hitch %s = %v, want the mistyped flag reported", flag, err)
		}
	}
	if options, err := parseHitch([]string{"worker", "-tset X=1", "-mprovider/model=x"}, "codex", "/work"); err != nil || options.Task != "set X=1" || options.Model != "provider/model=x" {
		t.Errorf("attached values with = = %+v, %v", options, err)
	}
}

// A failed agent's record keeps its name but no longer its pane, so a capture
// of it says that, rather than reading as a capture with no name.
func TestCaptureOfAgentWithoutPaneNamesIt(t *testing.T) {
	f := newStateFixture(t)
	f.add(t, "a", "worker", "codex")
	f.env["TMUX_PANE"] = "%1"
	p, err := f.run.team.Agent("a")
	if err != nil {
		t.Fatal(err)
	}
	l, err := p.LockAgent()
	if err != nil {
		t.Fatal(err)
	}
	a, err := p.Read()
	if err != nil {
		t.Fatal(err)
	}
	if err := f.run.forgetPane(l, &a); err != nil {
		t.Fatal(err)
	}
	if err := f.run.unlock(l); err != nil {
		t.Fatal(err)
	}
	f.out.Reset()
	requireRefused(t, f.cmd.capture([]string{"worker", "-n", "25"}), "worker has no pane to capture: it is failed")
	if f.out.Len() != 0 {
		t.Fatalf("captured %q", f.out.String())
	}
}
