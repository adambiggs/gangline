package main

import (
	"strings"
	"testing"
	"time"
)

// flagPackageText is wording only Go's flag package produces. None of it may
// reach a caller, who needs the accepted form instead.
var flagPackageText = []string{"flag provided but not defined", "bad flag syntax", "flag needs an argument", "parse error", "invalid value"}

// hitch reads team state before parsing, so it is checked at the parser.
func TestHitchErrorsStateTheAcceptedForm(t *testing.T) {
	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"worker", "-bogus"}, "hitch: unknown option -bogus"},
		{[]string{"worker", "-c"}, "hitch: -c needs a value (expected COLLAR)"},
		{[]string{"hitch"}, `invalid agent name "hitch" (expected letters, digits, '.', '_' or '-', starting with a letter or digit; hitch and gangline are reserved)`},
		{[]string{"bad name"}, `invalid agent name "bad name" (expected letters, digits, '.', '_' or '-', starting with a letter or digit; hitch and gangline are reserved)`},
	} {
		_, err := parseHitch(test.args, "claude", "/work")
		commandErr, ok := err.(commandError)
		if !ok || commandErr.status != exitUsage || commandErr.text != test.want {
			t.Errorf("parseHitch(%q) = %#v; want usage error %q", test.args, err, test.want)
		}
	}
}

func TestScheduleErrorsStateTheAcceptedForms(t *testing.T) {
	for _, value := range []string{"xyz", "-5m", ""} {
		if _, err := parseSchedule(value, time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC)); err != errScheduleForm {
			t.Errorf("parseSchedule(%q) = %v; want %v", value, err, errScheduleForm)
		}
	}
	f := newStateFixture(t)
	f.add(t, "a", "worker", "codex")
	f.cmd.stdin = strings.NewReader("hello")
	for _, test := range []struct {
		call func() error
		want string
	}{
		{func() error { return f.cmd.send([]string{"worker", "--from", "operator", "--at", "xyz"}) }, `send: invalid --at "xyz" (expected a positive DURATION such as 30m, HH:MM, or RFC3339)`},
		{func() error { return f.cmd.curfew([]string{"xyz"}) }, `curfew: invalid deadline "xyz" (expected a positive DURATION such as 30m, HH:MM, or RFC3339)`},
		{func() error {
			f.env["GANG_AGENT_ID"] = "a"
			defer delete(f.env, "GANG_AGENT_ID")
			return f.cmd.snooze([]string{"--at", "xyz"})
		}, `snooze: invalid --at "xyz" (expected a positive DURATION such as 30m, HH:MM, or RFC3339)`},
	} {
		err := test.call()
		commandErr, ok := err.(commandError)
		if !ok || commandErr.status != exitUsage || commandErr.text != test.want {
			t.Errorf("schedule error = %#v; want usage error %q", err, test.want)
		}
	}
}
