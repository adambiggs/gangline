package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adambiggs/gangline/harness"
)

func TestCollarCheckReportsProbesThatDidNotRunAsUnknown(t *testing.T) {
	f := newStateFixture(t)
	report := harness.CheckReport{Collar: "codex", HarnessVersion: "unknown"}
	unknown := unknownProbeResults()
	for _, name := range harness.RequiredProbes() {
		report.Results = append(report.Results, unknown[name])
	}
	err := f.cmd.printCheckReport(report)
	var ce commandError
	if !errors.As(err, &ce) || ce.status != exitUnknown {
		t.Fatalf("error = %v, want exit status %d", err, exitUnknown)
	}
	out := f.out.String()
	for _, name := range harness.RequiredProbes() {
		if !strings.Contains(out, "UNKNOWN\t"+name+"\t") {
			t.Fatalf("probe %s not reported unknown:\n%s", name, out)
		}
	}
	for _, guessed := range []string{"FAIL", "gh issue create", "incompatible", "native prompts"} {
		if strings.Contains(out+err.Error(), guessed) {
			t.Fatalf("report of unrun probes contains %q:\n%s\n%v", guessed, out, err)
		}
	}
}

// fakeCollarCheckTmux runs collar checks against a tmux stand-in whose
// subcommands are given as sh case arms; $socket holds the -S path.
func fakeCollarCheckTmux(t *testing.T, arms string) *stateFixture {
	t.Helper()
	fakeCodexOnPath(t)
	f := newStateFixture(t)
	fakeTmux := filepath.Join(t.TempDir(), "tmux")
	script := "#!/bin/sh\n# SPDX-License-Identifier: Apache-2.0\nsocket=$2\nshift 2\ncase \"$1\" in\n" + arms + "*) exit 91;;\nesac\n"
	if err := os.WriteFile(fakeTmux, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	f.env["GANG_TMUX"] = fakeTmux
	return f
}

func collarCheckExit(t *testing.T, err error, status int) {
	t.Helper()
	var ce commandError
	if !errors.As(err, &ce) || ce.status != status {
		t.Errorf("error = %v, want exit status %d", err, status)
	}
}

// A tmux server that cannot create the probe session leaves every probe
// unrun, and a session that cannot be stopped is a reported error that keeps
// its socket.
func TestCollarCheckReportsUnrunProbesAndCleanupFailures(t *testing.T) {
	f := fakeCollarCheckTmux(t, `new-session) mkdir -p "$socket"; echo "server refused session" >&2; exit 1;;
has-session) exit 0;;
kill-session) echo "kill refused" >&2; exit 1;;
`)
	err := f.cmd.collar([]string{"check", "codex"})
	if err == nil {
		t.Fatal("collar check with no session succeeded")
	}
	collarCheckExit(t, err, exitUnknown)
	out := f.out.String()
	if !strings.Contains(out, "UNKNOWN\tlaunch\t") || !strings.Contains(out, "server refused session") {
		t.Errorf("launch was not reported unknown with the tmux error:\n%s", out)
	}
	if strings.Contains(out, "FAIL") || strings.Contains(out, "gh issue create") {
		t.Errorf("unrun probes were reported as failures:\n%s", out)
	}
	for _, want := range []string{"kill refused", "stop tmux session gang-check-trust-", "-t.sock", "stop tmux session gang-check-active-", "-a.sock"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("cleanup failure lacks %q: %v", want, err)
		}
	}
	if sockets, _ := filepath.Glob(filepath.Join(f.env["GANG_STATE_ROOT"], ".local", "state", "gl-*.sock")); len(sockets) != 2 {
		t.Errorf("sockets of unstopped sessions = %v, want both kept", sockets)
	}
	if leftovers, _ := filepath.Glob(filepath.Join(f.env["GANG_STATE_ROOT"], "native-project-*")); len(leftovers) != 0 {
		t.Errorf("probe project left behind: %v", leftovers)
	}
}

func TestCollarCheckCleanupOfNothingIsNotAFailure(t *testing.T) {
	f := fakeCollarCheckTmux(t, `new-session) echo "server refused session" >&2; exit 1;;
has-session) echo "no server running on $socket" >&2; exit 1;;
kill-session) echo "kill-session reached" >&2; exit 1;;
`)
	err := f.cmd.collar([]string{"check", "codex"})
	collarCheckExit(t, err, exitUnknown)
	if want := "collar check incomplete; unknown: launch, trust-prompt, hook-fires, composer-detect, submit, turn-boundary"; err == nil || err.Error() != want {
		t.Errorf("error = %v, want %q", err, want)
	}
}

func TestCollarCheckTrustPromptLeavesHookedProbesUnknown(t *testing.T) {
	f := fakeCollarCheckTmux(t, `new-session) prev= prev2= generation=
	for argument; do [ "$prev2" = -soq ] && generation=$argument; prev2=$prev; prev=$argument; done
	printf '%%1\n%s\t$1\t%%1\n' "$generation";;
has-session) exit 1;;
`)
	f.input.screen = screenWithText("Trust this folder?", "› 1. Trust and continue")
	err := f.cmd.collar([]string{"check", "codex"})
	collarCheckExit(t, err, exitUnknown)
	out := f.out.String()
	for _, row := range []string{"PASS\tlaunch\t", "PASS\ttrust-prompt\tCodex folder trust is required", "UNKNOWN\tcomposer-detect\tnative trust prompt in ", "UNKNOWN\tsubmit\t"} {
		if !strings.Contains(out, row) {
			t.Errorf("output lacks %q:\n%s", row, out)
		}
	}
	if strings.Contains(out, "FAIL") {
		t.Errorf("trust-blocked probes reported as failures:\n%s", out)
	}
}

func TestCollarCheckCleanupFailureKeepsTheCheckStatus(t *testing.T) {
	cleanup := errors.New("stop tmux session s on /sock: kill refused")
	if err := withCleanupError(nil, cleanup); err == nil || err.(commandError).status != exitError {
		t.Fatalf("passing check with failed cleanup = %v, want exit status %d", err, exitError)
	}
	err := withCleanupError(commandError{status: exitNative, text: "collar check failed: submit"}, cleanup)
	if ce, ok := err.(commandError); !ok || ce.status != exitNative || ce.text != "collar check failed: submit; collar check cleanup failed: stop tmux session s on /sock: kill refused" {
		t.Fatalf("failed check with failed cleanup = %#v", err)
	}
	unknown := commandError{status: exitUnknown, text: "collar check incomplete; unknown: submit"}
	if err := withCleanupError(unknown, nil); err != unknown {
		t.Fatalf("clean cleanup changed the result: %v", err)
	}
	if err := withCleanupError(nil, nil); err != nil {
		t.Fatalf("clean passing check = %v", err)
	}
}

func TestCollarCheckClassifiesProbeErrors(t *testing.T) {
	tmuxFailure := exec.Command("false").Run()
	if tmuxFailure == nil {
		t.Fatal("false succeeded")
	}
	live := func(context.Context) (bool, error) { return true, nil }
	gone := func(context.Context) (bool, error) { return false, nil }
	unreadable := func(context.Context) (bool, error) { return false, errors.New("no tmux") }
	expired, cancel := context.WithCancel(context.Background())
	cancel()
	for _, test := range []struct {
		name    string
		ctx     context.Context
		err     error
		session func(context.Context) (bool, error)
		want    harness.ProbeOutcome
	}{
		{"instrument failure with live session", context.Background(), fmt.Errorf("capture pane: %w", tmuxFailure), live, harness.ProbeUnknown},
		{"instrument failure with unreadable session", context.Background(), fmt.Errorf("capture pane: %w", tmuxFailure), unreadable, harness.ProbeUnknown},
		{"native process exited", context.Background(), fmt.Errorf("capture pane: %w", tmuxFailure), gone, harness.ProbeFailed},
		{"unrecognized startup", context.Background(), errors.New("native startup was not observable within 8s"), live, harness.ProbeFailed},
		{"deadline", expired, fmt.Errorf("hook reader: %w", tmuxFailure), live, harness.ProbeFailed},
	} {
		if got := probeErrorResult(test.ctx, harness.ProbeSubmit, test.err, test.session); got.Outcome != test.want {
			t.Errorf("%s: outcome = %s, want %s", test.name, got.Outcome, test.want)
		}
	}
}

func TestCollarCheckOffersAnIssueOnlyForObservedFailures(t *testing.T) {
	f := newStateFixture(t)
	report := harness.CheckReport{Collar: "codex", HarnessVersion: "1.2.3"}
	unknown := unknownProbeResults()
	for _, name := range harness.RequiredProbes() {
		report.Results = append(report.Results, unknown[name])
	}
	report.Results[0] = passedProbe(harness.ProbeLaunch, "launched")
	report.Results[1] = failedProbe(harness.ProbeTrustPrompt, "startup not recognized")
	err := f.cmd.printCheckReport(report)
	var ce commandError
	if !errors.As(err, &ce) || ce.status != exitNative {
		t.Fatalf("error = %v, want exit status %d", err, exitNative)
	}
	if want := "collar check failed: trust-prompt; unknown: hook-fires, composer-detect, submit, turn-boundary"; err.Error() != want {
		t.Fatalf("error = %q, want %q", err, want)
	}
	out := f.out.String()
	for _, row := range []string{"PASS\tlaunch\t", "FAIL\ttrust-prompt\t", "UNKNOWN\thook-fires\t", "--title 'codex collar check failed on 1.2.3: trust-prompt'"} {
		if !strings.Contains(out, row) {
			t.Fatalf("output lacks %q:\n%s", row, out)
		}
	}
}

// The probe types into the native composer through the guarded input that
// real delivery uses, so an output viewer over the pane is dismissed first.
func TestCollarCheckTypesThroughGuardedInput(t *testing.T) {
	f := fakeCollarCheckTmux(t, `new-session) prev= prev2= generation= name=
	for argument; do [ "$prev2" = -soq ] && generation=$argument; [ "$prev" = -s ] && [ -z "$name" ] && name=$argument; prev2=$prev; prev=$argument; done
	printf '%s' "$generation" > "$socket.generation"
	printf '%s' "$name" > "$socket.session"
	printf '%%1\n%s\t$1\t%%1\n' "$generation";;
list-panes) printf '%s\t$1\t%%1\t%s\n' "$(cat "$socket.generation")" "$(cat "$socket.session")";;
display-message) printf 'codex\n';;
has-session) exit 0;;
kill-session) exit 0;;
if-shell|send-keys) printf '%s\n' "$@" > "$socket.input"; echo "stub accepts no input" >&2; exit 1;;
`)
	err := f.cmd.collar([]string{"check", "codex"})
	collarCheckExit(t, err, exitUnknown)
	inputs, _ := filepath.Glob(filepath.Join(f.env["GANG_STATE_ROOT"], ".local", "state", "gl-*-a.sock.input"))
	if len(inputs) != 1 {
		t.Fatalf("hooked probe input = %v, want one attempt\n%s", inputs, f.out.String())
	}
	data, readErr := os.ReadFile(inputs[0])
	if readErr != nil {
		t.Fatal(readErr)
	}
	sent := strings.Split(string(data), "\n")
	guarded := len(sent) > 5 && sent[0] == "if-shell" && strings.Contains(sent[4], "#{==:#{pane_current_command},codex}") && strings.HasPrefix(sent[5], `"if-shell" "-F" "-t" "%1" "#{||:#{==:#{pane_mode},copy-mode},#{==:#{pane_mode},view-mode}}" "\"send-keys\" \"-t\" \"%1\" \"-X\" \"cancel\"" ; "send-keys" "-t" "%1" "-l" "--" "`) && strings.Contains(sent[5], "Reply with exactly READY.")
	if !guarded {
		t.Fatalf("probe input bypassed guarded input:\n%s", data)
	}
	if out := f.out.String(); !strings.Contains(out, "UNKNOWN\tsubmit\t") || !strings.Contains(out, "stub accepts no input") {
		t.Fatalf("refused input not reported as unknown submit:\n%s", out)
	}
}

// A native process that exits before its pane is registered for input is an
// observed failure, not an unknown.
func TestCollarCheckReportsAnExitBeforeRegistrationAsFailure(t *testing.T) {
	f := fakeCollarCheckTmux(t, `new-session) prev= prev2= generation=
	for argument; do [ "$prev2" = -soq ] && generation=$argument; prev2=$prev; prev=$argument; done
	printf '%%1\n%s\t$1\t%%1\n' "$generation";;
list-panes) printf 'no server running on %s\n' "$socket" >&2; exit 1;;
has-session) printf 'no server running on %s\n' "$socket" >&2; exit 1;;
`)
	err := f.cmd.collar([]string{"check", "codex"})
	collarCheckExit(t, err, exitNative)
	if out := f.out.String(); !strings.Contains(out, "FAIL\tsubmit\tnative process exited: created pane %1 is absent") {
		t.Errorf("exit before registration not reported as a failed submit:\n%s", out)
	}
}
